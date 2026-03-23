package httpkit

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os/exec"
	"sync"
	"time"
)

// ProxyConfig holds proxy and VPN rotation settings.
type ProxyConfig struct {
	URL              string
	RotationEnabled  bool
	RotateEveryN     int
	RotateInterval   time.Duration
	MullvadCLI       bool
	MullvadLocations []string
}

// ProxyTransport wraps http.Transport with optional proxy and rotation.
func ProxyTransport(cfg ProxyConfig) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if cfg.URL != "" {
		proxyURL, err := url.Parse(cfg.URL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return transport
}

// ProxyEnvVars returns environment variables for child processes.
func ProxyEnvVars(proxyURL string) []string {
	if proxyURL == "" {
		return nil
	}
	return []string{
		"HTTP_PROXY=" + proxyURL,
		"HTTPS_PROXY=" + proxyURL,
		"ALL_PROXY=" + proxyURL,
	}
}

// MullvadRotator rotates Mullvad VPN connections on a schedule.
type MullvadRotator struct {
	locations    []string
	interval     time.Duration
	everyN       int
	requestCount int
	mu           sync.Mutex
	stopCh       chan struct{}
}

// NewMullvadRotator creates a rotator. Call Start() to begin.
func NewMullvadRotator(locations []string, interval time.Duration, everyN int) *MullvadRotator {
	if len(locations) == 0 {
		locations = []string{"us", "de", "nl", "se", "ch", "gb", "jp"}
	}
	if everyN <= 0 {
		everyN = 50
	}
	return &MullvadRotator{
		locations: locations,
		interval:  interval,
		everyN:    everyN,
		stopCh:    make(chan struct{}),
	}
}

// Start begins time-based rotation in background.
func (m *MullvadRotator) Start() {
	if m.interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.Rotate()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop stops background rotation.
func (m *MullvadRotator) Stop() {
	close(m.stopCh)
}

// OnRequest should be called before each request. Rotates after N requests.
func (m *MullvadRotator) OnRequest() {
	m.mu.Lock()
	m.requestCount++
	shouldRotate := m.requestCount >= m.everyN
	if shouldRotate {
		m.requestCount = 0
	}
	m.mu.Unlock()

	if shouldRotate {
		m.Rotate()
	}
}

// Rotate switches to a random Mullvad relay location.
func (m *MullvadRotator) Rotate() {
	loc := m.locations[rand.Intn(len(m.locations))]
	log.Printf("[mullvad] Rotating to %s...", loc)

	// Set relay location
	if err := exec.Command("mullvad", "relay", "set", "location", loc).Run(); err != nil {
		log.Printf("[mullvad] Failed to set location: %v", err)
		return
	}

	// Reconnect
	if err := exec.Command("mullvad", "reconnect").Run(); err != nil {
		log.Printf("[mullvad] Failed to reconnect: %v", err)
		return
	}

	// Wait for connection to establish
	time.Sleep(3 * time.Second)

	// Verify
	out, err := exec.Command("mullvad", "status").Output()
	if err == nil {
		log.Printf("[mullvad] %s", string(out))
	}
}

// Status checks if Mullvad is connected.
func MullvadStatus() (string, error) {
	out, err := exec.Command("mullvad", "status").Output()
	if err != nil {
		return "", fmt.Errorf("mullvad not available: %w", err)
	}
	return string(out), nil
}
