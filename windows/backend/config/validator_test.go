package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func testRuntimeConfig() []byte {
	return mustJSON(map[string]any{
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"tag": "cloudflare", "type": "https", "detour": "auto"},
				map[string]any{"tag": "local", "type": "local", "detour": "direct"},
			},
		},
		"outbounds": []any{
			map[string]any{"type": "urltest", "tag": "auto", "outbounds": []any{"vps-a", "vps-b", "direct"}},
			map[string]any{"type": "hysteria2", "tag": "vps-a", "server": "198.51.100.10", "server_port": 8443},
			map[string]any{"type": "vless", "tag": "vps-b", "server": "203.0.113.10", "server_port": 443},
			map[string]any{"type": "direct", "tag": "direct"},
			map[string]any{"type": "block", "tag": "block"},
		},
		"route": map[string]any{
			"rules": []any{map[string]any{"domain_suffix": []any{"example.com"}, "outbound": "auto"}},
			"final": "auto",
		},
	})
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func TestNormalizeProtectedRouteCreatesSelectorWithoutDirect(t *testing.T) {
	normalized, err := NormalizeProtectedRoute(testRuntimeConfig())
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := Validate(normalized, ValidationOptions{RequireFailClosed: true}); err != nil {
		t.Fatalf("normalized config should validate: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(normalized, &root); err != nil {
		t.Fatal(err)
	}
	var selector map[string]any
	for _, raw := range root["outbounds"].([]any) {
		item := raw.(map[string]any)
		if item["tag"] == "proxy" {
			selector = item
		}
	}
	if selector == nil {
		t.Fatal("proxy selector was not created")
	}
	for _, raw := range selector["outbounds"].([]any) {
		if raw == "direct" {
			t.Fatal("direct must not be in protected selector")
		}
	}
	if root["route"].(map[string]any)["final"] != "proxy" {
		t.Fatal("route.final must point to proxy selector")
	}
}

func TestValidateRejectsPlaceholders(t *testing.T) {
	cfg := mustJSON(map[string]any{
		"outbounds": []any{map[string]any{"type": "vless", "tag": "vps", "server": "YOUR_VPS_IP"}},
		"route":     map[string]any{"final": "vps"},
	})
	if err := Validate(cfg, ValidationOptions{}); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("expected placeholder validation error, got %v", err)
	}
}

func TestValidateRejectsMissingReference(t *testing.T) {
	cfg := mustJSON(map[string]any{
		"outbounds": []any{map[string]any{"type": "selector", "tag": "proxy", "outbounds": []any{"missing"}}},
		"route":     map[string]any{"final": "proxy"},
	})
	if err := Validate(cfg, ValidationOptions{}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected missing reference error, got %v", err)
	}
}

func TestValidateRejectsDirectProtectedSelector(t *testing.T) {
	cfg := mustJSON(map[string]any{
		"outbounds": []any{
			map[string]any{"type": "selector", "tag": "proxy", "outbounds": []any{"vps", "direct"}, "default": "vps"},
			map[string]any{"type": "hysteria2", "tag": "vps", "server": "198.51.100.10", "server_port": 8443},
			map[string]any{"type": "direct", "tag": "direct"},
		},
		"route": map[string]any{"final": "proxy"},
	})
	if err := Validate(cfg, ValidationOptions{RequireFailClosed: true}); err == nil || !strings.Contains(err.Error(), "cannot contain direct") {
		t.Fatalf("expected fail-closed selector error, got %v", err)
	}
}

func TestValidateAllowsExplicitDirectRoute(t *testing.T) {
	cfg := mustJSON(map[string]any{
		"outbounds": []any{
			map[string]any{"type": "selector", "tag": "proxy", "outbounds": []any{"vps"}, "default": "vps"},
			map[string]any{"type": "hysteria2", "tag": "vps", "server": "198.51.100.10", "server_port": 8443},
			map[string]any{"type": "direct", "tag": "direct"},
		},
		"route": map[string]any{
			"rules": []any{map[string]any{"domain_suffix": []any{"ru"}, "outbound": "direct"}},
			"final": "proxy",
		},
	})
	if err := Validate(cfg, ValidationOptions{RequireFailClosed: true}); err != nil {
		t.Fatalf("explicit RU direct route should be allowed: %v", err)
	}
}
