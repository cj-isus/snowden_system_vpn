package core

import (
	"encoding/json"
	"fmt"
	"testing"
)

func testCfgWith(tags ...map[string]string) []byte {
	out := make([]map[string]any, 0, len(tags))
	for _, t := range tags {
		ob := map[string]any{"type": t["type"], "tag": t["tag"]}
		if s, ok := t["server"]; ok {
			ob["server"] = s
		}
		if s, ok := t["server_port"]; ok {
			var p int
			fmt.Sscanf(s, "%d", &p)
			ob["server_port"] = p
		}
		out = append(out, ob)
	}
	b, err := json.Marshal(map[string]any{
		"outbounds": out,
		"route":     map[string]any{"final": "auto"},
	})
	if err != nil {
		panic(err)
	}
	return b
}

func TestResolveOutboundTagAuto(t *testing.T) {
	cfg := testCfgWith(
		map[string]string{"type": "hysteria2", "tag": "hysteria2-nl", "server": "1.2.3.4", "server_port": "8443"},
		map[string]string{"type": "urltest", "tag": "auto"},
		map[string]string{"type": "direct", "tag": "direct"},
	)
	tag, err := ResolveOutboundTag("auto", cfg)
	if err != nil || tag != "auto" {
		t.Fatalf("auto → got %q, err %v", tag, err)
	}
}

func TestResolveOutboundTagByCountry(t *testing.T) {
	cfg := testCfgWith(
		map[string]string{"type": "hysteria2", "tag": "hysteria2-nl", "server": "1.2.3.4", "server_port": "8443"},
		map[string]string{"type": "vless", "tag": "vless-fr", "server": "5.6.7.8", "server_port": "443"},
		map[string]string{"type": "direct", "tag": "direct"},
	)
	tag, err := ResolveOutboundTag("nl", cfg)
	if err != nil || tag != "hysteria2-nl" {
		t.Fatalf("nl → got %q, err %v", tag, err)
	}
	tag, err = ResolveOutboundTag("fr", cfg)
	if err != nil || tag != "vless-fr" {
		t.Fatalf("fr → got %q, err %v", tag, err)
	}
}

func TestResolveOutboundTagRenamedStillMatches(t *testing.T) {
	// Tag retains the country code even after a rename (prefix/suffix changed),
	// e.g. vless-nl → prod-nl-vless: "nl" is still found by contains.
	cfg := testCfgWith(
		map[string]string{"type": "vless", "tag": "prod-nl-vless", "server": "1.2.3.4", "server_port": "443"},
		map[string]string{"type": "direct", "tag": "direct"},
	)
	tag, err := ResolveOutboundTag("nl", cfg)
	if err != nil || tag != "prod-nl-vless" {
		t.Fatalf("nl (renamed) → got %q, err %v", tag, err)
	}
}

func TestResolveOutboundTagUnknownReturnsError(t *testing.T) {
	cfg := testCfgWith(
		map[string]string{"type": "hysteria2", "tag": "hysteria2-nl", "server": "1.2.3.4", "server_port": "8443"},
	)
	if _, err := ResolveOutboundTag("xx", cfg); err == nil {
		t.Fatal("unknown country must return an error, not silently succeed")
	}
}

func TestResolveOutboundTagNoURLTest(t *testing.T) {
	cfg := testCfgWith(
		map[string]string{"type": "hysteria2", "tag": "hysteria2-nl", "server": "1.2.3.4", "server_port": "8443"},
	)
	if _, err := ResolveOutboundTag("auto", cfg); err == nil {
		t.Fatal("missing urltest must return an error")
	}
}
