package httpkit

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"reconx/internal/config"
)

// Client wraps http.Client with per-host rate limiting, retry/backoff, and proxy support.
type Client struct {
	inner       *http.Client
	rateLimiter *HostRateLimiter
	maxRetries  int
	rotator     *MullvadRotator
}

// NewClient creates an HTTP client from config with rate limiting, proxy, and retry.
func NewClient(cfg *config.Config) *Client {
	rps := cfg.General.RateLimitPerHost
	if rps <= 0 {
		rps = 10
	}
	maxRetries := cfg.General.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	proxyCfg := ProxyConfig{
		URL:              cfg.Proxy.URL,
		RotationEnabled:  cfg.Proxy.RotationEnabled,
		MullvadCLI:       cfg.Proxy.MullvadCLI,
		MullvadLocations: cfg.Proxy.MullvadLocations,
		RotateEveryN:     cfg.Proxy.RotateEveryN,
	}
	if cfg.Proxy.RotateInterval != "" {
		if d, err := time.ParseDuration(cfg.Proxy.RotateInterval); err == nil {
			proxyCfg.RotateInterval = d
		}
	}

	transport := ProxyTransport(proxyCfg)

	inner := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	c := &Client{
		inner:       inner,
		rateLimiter: NewHostRateLimiter(rps),
		maxRetries:  maxRetries,
	}

	// Start Mullvad rotator if configured
	if proxyCfg.MullvadCLI && proxyCfg.RotationEnabled {
		c.rotator = NewMullvadRotator(proxyCfg.MullvadLocations, proxyCfg.RotateInterval, proxyCfg.RotateEveryN)
		c.rotator.Start()
	}

	return c
}

// NewClientWithTimeout creates a client with a custom timeout.
func NewClientWithTimeout(cfg *config.Config, timeout time.Duration) *Client {
	c := NewClient(cfg)
	c.inner.Timeout = timeout
	return c
}

// NewClientWithRedirects creates a client with redirect control.
func NewClientWithRedirects(cfg *config.Config, maxRedirects int) *Client {
	c := NewClient(cfg)
	c.inner.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return c
}

// Do executes a request with rate limiting, retry, and backoff.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Wait for rate limit
	if err := c.rateLimiter.Wait(req.Context(), req.URL.String()); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	// Mullvad rotation
	if c.rotator != nil {
		c.rotator.OnRequest()
	}

	var lastErr error
	backoff := time.Second

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[httpkit] retry %d/%d for %s after %s", attempt, c.maxRetries, req.URL.Host, backoff)
			select {
			case <-time.After(backoff):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}

		resp, err := c.inner.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Retry on 429 or 503
		if resp.StatusCode == 429 || resp.StatusCode == 503 {
			// Check Retry-After header
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					backoff = time.Duration(secs) * time.Second
				}
			}
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, req.URL.Host)
			log.Printf("[httpkit] %s — backing off %s", lastErr, backoff)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded for %s: %w", req.URL.Host, lastErr)
}

// Close cleans up resources (stops rotator if running).
func (c *Client) Close() {
	if c.rotator != nil {
		c.rotator.Stop()
	}
}
