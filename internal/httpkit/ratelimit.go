package httpkit

import (
	"context"
	"net/url"
	"sync"

	"golang.org/x/time/rate"
)

// HostRateLimiter enforces per-host request rate limits using token buckets.
type HostRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      int
}

// NewHostRateLimiter creates a rate limiter with the given requests-per-second per host.
func NewHostRateLimiter(rps int) *HostRateLimiter {
	if rps <= 0 {
		rps = 10
	}
	return &HostRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rps,
	}
}

// Wait blocks until the rate limit allows a request to the given host.
func (h *HostRateLimiter) Wait(ctx context.Context, rawURL string) error {
	host := extractHost(rawURL)
	limiter := h.getLimiter(host)
	return limiter.Wait(ctx)
}

func (h *HostRateLimiter) getLimiter(host string) *rate.Limiter {
	h.mu.Lock()
	defer h.mu.Unlock()

	if l, ok := h.limiters[host]; ok {
		return l
	}

	l := rate.NewLimiter(rate.Limit(h.rps), h.rps) // burst = rps
	h.limiters[host] = l
	return l
}

func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Hostname()
}
