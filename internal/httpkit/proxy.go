package httpkit

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
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
// It uses `mullvad reconnect --wait` so the OS-level tunnel is confirmed
// before returning, eliminating the need for a fixed sleep.
func (m *MullvadRotator) Rotate() {
	loc := m.locations[rand.Intn(len(m.locations))]
	log.Printf("[mullvad] Rotating to %s...", loc)

	// 1. Set relay location
	if out, err := exec.Command("mullvad", "relay", "set", "location", loc).CombinedOutput(); err != nil {
		log.Printf("[mullvad] Failed to set location %s: %v — %s", loc, err, strings.TrimSpace(string(out)))
		return
	}

	// 2. Reconnect and wait until the tunnel is established (no sleep needed).
	//    --wait blocks until connected or times out internally (~30s).
	if out, err := exec.Command("mullvad", "reconnect", "--wait").CombinedOutput(); err != nil {
		log.Printf("[mullvad] Failed to reconnect: %v — %s", err, strings.TrimSpace(string(out)))
		return
	}

	// 3. Log final status
	if status, err := MullvadStatus(); err == nil {
		log.Printf("[mullvad] %s", status)
	}
}

// MullvadStatus returns the current Mullvad connection status string.
func MullvadStatus() (string, error) {
	out, err := exec.Command("mullvad", "status").Output()
	if err != nil {
		return "", fmt.Errorf("mullvad not available: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// MullvadIsConnected returns true when Mullvad reports it is connected.
func MullvadIsConnected() bool {
	status, err := MullvadStatus()
	if err != nil {
		return false
	}
	lower := strings.ToLower(status)
	return strings.Contains(lower, "connected") && !strings.Contains(lower, "disconnected")
}

// MullvadConnect ensures Mullvad is connected. It connects if not already
// connected and waits up to maxWait for the tunnel to come up.
func MullvadConnect(maxWait time.Duration) error {
	// Already connected — nothing to do.
	if MullvadIsConnected() {
		return nil
	}

	log.Printf("[mullvad] Not connected — triggering connect...")
	if out, err := exec.Command("mullvad", "connect", "--wait").CombinedOutput(); err != nil {
		return fmt.Errorf("mullvad connect failed: %w — %s", err, strings.TrimSpace(string(out)))
	}

	// Confirm the connection came up.
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if MullvadIsConnected() {
			status, _ := MullvadStatus()
			log.Printf("[mullvad] Connected: %s", status)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("mullvad did not connect within %s", maxWait)
}
