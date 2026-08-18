// Package cfclient fetches dynamic VPN configurations and health-check data
// from the snowden.system Cloudflare Worker.
//
// This lets the app update server lists, protocols, and endpoints WITHOUT
// rebuilding the .exe — just update the KV store on Cloudflare.
package cfclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// DefaultWorkerURL is the Cloudflare Worker endpoint.
	// Override at runtime with the SNOWDEN_WORKER_URL env var,
	// or programmatically via SetWorkerURL().
	DefaultWorkerURL = "https://snowden-system-api.pcel628.workers.dev"
	httpTimeout      = 10 * time.Second
)

// Client fetches data from the snowden.system Cloudflare Worker.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a Client pointing to the Worker URL.
// Resolution order: SNOWDEN_WORKER_URL env var → DefaultWorkerURL.
func New() *Client {
	base := DefaultWorkerURL
	if env := os.Getenv("SNOWDEN_WORKER_URL"); env != "" {
		base = env
	}
	return &Client{
		baseURL: base,
		http:    &http.Client{Timeout: httpTimeout},
	}
}

// SetWorkerURL overrides the Worker URL (e.g. for a custom domain).
func (c *Client) SetWorkerURL(url string) {
	c.baseURL = url
}

// ServerConfig describes a single VPN server from the dynamic config.
type ServerConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Port     uint16 `json:"port"`
	Protocol string `json:"protocol"`
	Domain   string `json:"domain"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	PublicKey string `json:"publicKey,omitempty"`
	Active   bool   `json:"active"`
}

// DynamicConfig is the full config returned by GET /api/config.
type DynamicConfig struct {
	Servers []ServerConfig `json:"servers"`
	Routing struct {
		RUCidrURL      string `json:"ruCidrUrl"`
		SplitTunneling bool   `json:"splitTunneling"`
	} `json:"routing"`
	Version   string `json:"version"`
	UpdatedAt string `json:"updatedAt"`
}

// HealthResult is returned by GET /api/health.
type HealthResult struct {
	Edge      string                 `json:"edge"`
	Timestamp string                 `json:"timestamp"`
	Tests     map[string]interface{} `json:"tests"`
}

// VersionInfo is returned by GET /api/version.
type VersionInfo struct {
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
}

// FetchConfig retrieves the dynamic VPN config from the Worker.
// Falls back gracefully if the Worker is unreachable.
func (c *Client) FetchConfig() (*DynamicConfig, error) {
	resp, err := c.http.Get(c.baseURL + "/api/config")
	if err != nil {
		return nil, fmt.Errorf("fetch config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("config: HTTP %d", resp.StatusCode)
	}
	var cfg DynamicConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return &cfg, nil
}

// FetchHealth retrieves the VPS health-check from the CF edge.
func (c *Client) FetchHealth() (*HealthResult, error) {
	resp, err := c.http.Get(c.baseURL + "/api/health")
	if err != nil {
		return nil, fmt.Errorf("fetch health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("health: HTTP %d", resp.StatusCode)
	}
	var h HealthResult
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return nil, fmt.Errorf("decode health: %w", err)
	}
	return &h, nil
}

// FetchVersion checks for app updates.
func (c *Client) FetchVersion() (*VersionInfo, error) {
	resp, err := c.http.Get(c.baseURL + "/api/version")
	if err != nil {
		return nil, fmt.Errorf("fetch version: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("version: HTTP %d", resp.StatusCode)
	}
	var v VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode version: %w", err)
	}
	return &v, nil
}

// SendTelemetry posts an anonymous event to the D1 database via the Worker.
func (c *Client) SendTelemetry(region, event, protocol string, latencyMs int) error {
	body, _ := json.Marshal(map[string]any{
		"region":     region,
		"event":      event,
		"protocol":   protocol,
		"latency_ms": latencyMs,
	})
	resp, err := c.http.Post(
		c.baseURL+"/api/telemetry",
		"application/json",
		bytesReader(body),
	)
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	resp.Body.Close()
	return nil
}

// bytesReader is a minimal io.Reader from a byte slice (avoids importing bytes).
type byteReader struct {
	data []byte
	pos  int
}

func bytesReader(b []byte) *byteReader { return &byteReader{data: b} }
func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
