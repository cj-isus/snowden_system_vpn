package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	runtimeconfig "snowden-system/backend/config"
)

// ChannelDescriptor is the public, secret-free description of one protected
// outbound. It is derived from the actual config; no country or protocol list
// is hardcoded in the UI/controller.
type ChannelDescriptor struct {
	ID       string `json:"id"`
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Country  string `json:"country"`
	Profile  string `json:"profile"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// ProtectedChannels returns only the candidates owned by selector "proxy".
// If a legacy config has no selector yet, it returns real server outbounds so
// diagnostics can still explain what normalization must do.
func ProtectedChannels(configJSON []byte) []ChannelDescriptor {
	var root struct {
		Outbounds []struct {
			Type       string   `json:"type"`
			Tag        string   `json:"tag"`
			Server     string   `json:"server"`
			ServerPort int      `json:"server_port"`
			Outbounds  []string `json:"outbounds"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(configJSON, &root); err != nil {
		return nil
	}
	byTag := make(map[string]struct {
		typ, tag, server string
		port             int
	})
	for _, outbound := range root.Outbounds {
		byTag[outbound.Tag] = struct {
			typ, tag, server string
			port             int
		}{outbound.Type, outbound.Tag, outbound.Server, outbound.ServerPort}
	}

	var candidateTags []string
	for _, outbound := range root.Outbounds {
		if outbound.Type == "selector" && outbound.Tag == "proxy" {
			candidateTags = outbound.Outbounds
			break
		}
	}
	if len(candidateTags) == 0 {
		for _, outbound := range root.Outbounds {
			if isProtectedOutbound(outbound.Type, outbound.Server) {
				candidateTags = append(candidateTags, outbound.Tag)
			}
		}
	}

	result := make([]ChannelDescriptor, 0, len(candidateTags))
	for i, tag := range candidateTags {
		ob, ok := byTag[tag]
		if !ok || !isProtectedOutbound(ob.typ, ob.server) {
			continue
		}
		result = append(result, ChannelDescriptor{
			ID:       ob.tag,
			Tag:      ob.tag,
			Type:     ob.typ,
			Server:   ob.server,
			Port:     ob.port,
			Country:  countryFromTag(ob.tag),
			Profile:  ob.typ,
			Enabled:  true,
			Priority: i,
		})
	}
	return result
}

// SelectedChannelTag returns the actual selector default, not a guessed first
// server. Empty means the config has no selector-owned choice.
func SelectedChannelTag(configJSON []byte) string {
	var root struct {
		Outbounds []struct {
			Type    string `json:"type"`
			Tag     string `json:"tag"`
			Default string `json:"default"`
		} `json:"outbounds"`
	}
	if json.Unmarshal(configJSON, &root) != nil {
		return ""
	}
	for _, outbound := range root.Outbounds {
		if outbound.Type == "selector" && outbound.Tag == "proxy" {
			return outbound.Default
		}
	}
	return ""
}

// SelectedChannelKey returns the memory key for the channel actually selected
// by the protected selector.
func SelectedChannelKey(configJSON []byte) string {
	selected := SelectedChannelTag(configJSON)
	for _, channel := range ProtectedChannels(configJSON) {
		if channel.Tag == selected {
			return ChannelKey(channel)
		}
	}
	return ""
}

// ChannelKey is deterministic and intentionally hashes the endpoint. It keeps
// protocol/profile context without putting a server address or credential into
// the persisted diagnostics file.
func ChannelKey(channel ChannelDescriptor) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", channel.Server, channel.Type, channel.Port)))
	return fmt.Sprintf("%s:%s:%s", channel.Type, channel.ID, hex.EncodeToString(hash[:8]))
}

// ApplyChannel creates a new config snapshot with selector "proxy" pointing
// at the requested validated protected channel. Direct/block tags can never be
// selected through this function.
func ApplyChannel(configJSON []byte, channelTag string) ([]byte, error) {
	if channelTag == "" {
		return nil, fmt.Errorf("channel tag is empty")
	}
	known := false
	for _, channel := range ProtectedChannels(configJSON) {
		if channel.Tag == channelTag {
			known = true
			break
		}
	}
	if !known {
		return nil, fmt.Errorf("protected channel %q is not available", channelTag)
	}
	return runtimeconfig.SetProtectedSelectorDefault(configJSON, channelTag)
}

func isProtectedOutbound(typ, server string) bool {
	if server == "" {
		return false
	}
	switch typ {
	case "direct", "block", "dns", "selector", "urltest", "socks", "http":
		return false
	default:
		return true
	}
}

func countryFromTag(tag string) string {
	lower := strings.ToLower(tag)
	for code, country := range map[string]string{
		"nl": "NL",
		"fr": "FR",
		"de": "DE",
		"fi": "FI",
		"us": "US",
		"ru": "RU",
	} {
		if strings.Contains(lower, "-"+code) || strings.HasSuffix(lower, code) {
			return country
		}
	}
	return ""
}
