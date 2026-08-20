// Package config is LEGACY. The active config path is app.LoadConfigFile /
// app.ResolveOutboundTag reading template JSON from configs/ → assets/configs/.
//
// This package now only carries the VPSConfig value type (describing the
// personal VLESS server) for reference. BuildConfig / EnsureCIDRFile /
// splitCSV were dead code (the RU-CIDR split was removed for performance) and
// have been deleted — see PLAN.md A4.
package config

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
