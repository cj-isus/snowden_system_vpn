// Package config builds the sing-box JSON configuration used by the engine.
// It is the single source of truth for the outbound chain, DNS, routing rules
// and split-tunneling policy.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// VPSConfig carries the parameters of the user's personal VLESS server.
type VPSConfig struct {
	Server      string `json:"server"`
	ServerPort  uint16 `json:"server_port"`
	UUID        string `json:"uuid"`
	Flow        string `json:"flow"`
	ServerName  string `json:"server_name"`
	Insecure    bool   `json:"insecure"`
	Fingerprint string `json:"fingerprint"`
}

// BuildConfig assembles a full sing-box config JSON for the engine.
// If ruCIDRPath points to a file, it is registered as a local rule_set so
// Russian IPs go direct (split-tunneling) and everything else via proxy.
func BuildConfig(vps VPSConfig, listenPort uint16, ruCIDRPath string) ([]byte, error) {
	cfg := map[string]any{
		"log": map[string]any{"level": "info", "timestamp": true},
		"dns": map[string]any{
			"servers": []map[string]any{
				{"type": "https", "tag": "cloudflare", "server": "1.1.1.1", "path": "/dns-query", "detour": "auto"},
				{"type": "local", "tag": "local", "detour": "direct"},
			},
			"rules":    []map[string]any{{"outbound": "any", "server": "local"}},
			"strategy": "ipv4_only",
		},
		"inbounds": []map[string]any{
			{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": listenPort},
		},
		"outbounds": []map[string]any{
			{
				"type": "urltest", "tag": "auto",
				"outbounds": []string{"proxy", "direct"},
				"url":       "https://www.gstatic.com/generate_204",
				"interval":  "1m",
				"tolerance": 100,
			},
			{
				"type": "vless", "tag": "proxy",
				"server": vps.Server, "server_port": vps.ServerPort,
				"uuid": vps.UUID,
				"tls": map[string]any{
					"enabled":     true,
					"server_name": vps.ServerName,
				},
			},
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
	}

	rules := []map[string]any{
		{"action": "sniff"},
		{"action": "hijack-dns", "inbound": "mixed-in", "protocol": "dns"},
	}

	// Split-tunneling via local rule_set if the CIDR file exists.
	var ruleSets []map[string]any
	if ruCIDRPath != "" {
		if _, err := os.Stat(ruCIDRPath); err == nil {
			ruleSets = append(ruleSets, map[string]any{
				"type":   "local",
				"tag":    "ru-cidr",
				"format": "source",
				"path":   ruCIDRPath,
			})
			rules = append(rules, map[string]any{
				"rule_set": []string{"ru-cidr"},
				"action":   "direct",
			})
		}
	}

	rules = append(rules, map[string]any{"ip_is_private": true, "action": "direct"})

	route := map[string]any{
		"rules":                   rules,
		"final":                   "proxy",
		"default_domain_resolver": "local",
		"auto_detect_interface":   true,
	}
	if len(ruleSets) > 0 {
		route["rule_set"] = ruleSets
	}
	cfg["route"] = route

	return json.MarshalIndent(cfg, "", "  ")
}

// EnsureCIDRFile converts the comma-separated CIDR list to the sing-box
// "source" rule-set format (JSON with ip_cidr) and writes it next to the
// executable. Returns the path to the generated file.
func EnsureCIDRFile(rawList string, dir string) (string, error) {
	if rawList == "" {
		return "", nil
	}
	path := filepath.Join(dir, "ru-cidr.json")
	// sing-box source format is JSON: {"version": 3, "rules": [{"ip_cidr": [...]}]}
	rs := map[string]any{
		"version": 3,
		"rules":   []map[string]any{{"ip_cidr": splitCSV(rawList)}},
	}
	data, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write CIDR file: %w", err)
	}
	return path, nil
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		// Split by comma, newline, or carriage return
		if s[i] == ',' || s[i] == '\n' || s[i] == '\r' {
			if start < i {
				out = append(out, trim(s[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, trim(s[start:]))
	}
	return out
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
