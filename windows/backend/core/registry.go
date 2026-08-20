package core

import (
	"context"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter/certificate"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/dns/transport/fakeip"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/protocol/block"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing-box/protocol/http"
	"github.com/sagernet/sing-box/protocol/hysteria"
	"github.com/sagernet/sing-box/protocol/hysteria2"
	"github.com/sagernet/sing-box/protocol/masque"
	"github.com/sagernet/sing-box/protocol/mixed"
	"github.com/sagernet/sing-box/protocol/shadowsocks"
	"github.com/sagernet/sing-box/protocol/shadowtls"
	"github.com/sagernet/sing-box/protocol/socks"
	"github.com/sagernet/sing-box/protocol/trojan"
	"github.com/sagernet/sing-box/protocol/tun"
	"github.com/sagernet/sing-box/protocol/vless"
	"github.com/sagernet/sing-box/protocol/vmess"
	"github.com/sagernet/sing-box/protocol/wireguard"
	"github.com/sagernet/sing-box/service/resolved"
)

// boxContext builds a sing-box registry context for sing-box-lx (v1.14.0-lx).
//
// lx philosophy: features live behind build tags. WireGuard endpoint compiled
// with `with_awg` exposes AmneziaWG 2.0 (see transport/wireguard/device_awg.go);
// MASQUE outbound is always on (protocol/masque). We register exactly the
// protocols this app uses and deliberately skip protocol/naive (which would
// pull sagernet/cronet-go/all + prebuilt tvOS/iOS binaries that fail to
// download through RU-restricted proxies).
func boxContext(parent context.Context) context.Context {
	return box.Context(
		parent,
		inbounds(),
		outbounds(),
		endpoints(),
		dnsTransports(),
		services(),
		certificateProviders(),
	)
}

func inbounds() *inbound.Registry {
	r := inbound.NewRegistry()
	tun.RegisterInbound(r)
	direct.RegisterInbound(r)
	mixed.RegisterInbound(r)
	socks.RegisterInbound(r)
	http.RegisterInbound(r)
	shadowsocks.RegisterInbound(r)
	return r
}

func outbounds() *outbound.Registry {
	r := outbound.NewRegistry()
	direct.RegisterOutbound(r)
	block.RegisterOutbound(r)

	// Selector + url-test = the proxy-groups the adaptive engine switches between.
	group.RegisterSelector(r)
	group.RegisterURLTest(r)

	// Plain proxies.
	socks.RegisterOutbound(r)
	http.RegisterOutbound(r)
	shadowsocks.RegisterOutbound(r)
	vmess.RegisterOutbound(r)
	trojan.RegisterOutbound(r)
	shadowtls.RegisterOutbound(r)
	vless.RegisterOutbound(r)

	// MASQUE (CONNECT-IP / Cloudflare WARP) — lx SPEC 021. Lets the app reach
	// the internet for free through Cloudflare without a private VPS.
	masque.RegisterOutbound(r)

	// QUIC-based: Hysteria2 (+Gecko obfuscation in sing-box ≥ 1.14) is a core
	// part of the bypass stack. Hysteria v1 is registered for completeness.
	hysteria.RegisterOutbound(r)
	hysteria2.RegisterOutbound(r)

	return r
}

func endpoints() *endpoint.Registry {
	r := endpoint.NewRegistry()
	// WireGuard endpoint — when built with the `with_awg` tag this same call
	// also enables AmneziaWG 2.0 (it is implemented inside the wireguard
	// transport, not as a separate outbound). See device_awg.go.
	wireguard.RegisterEndpoint(r)
	return r
}

func dnsTransports() *dns.TransportRegistry {
	r := dns.NewTransportRegistry()
	transport.RegisterTCP(r)
	transport.RegisterUDP(r)
	transport.RegisterTLS(r)
	transport.RegisterHTTPS(r)
	hosts.RegisterTransport(r)
	local.RegisterTransport(r)
	fakeip.RegisterTransport(r)
	resolved.RegisterTransport(r)
	return r
}

func services() *service.Registry {
	r := service.NewRegistry()
	resolved.RegisterService(r)
	return r
}

func certificateProviders() *certificate.Registry {
	// lx 1.14.0 requires a CertificateProviderRegistry as the 7th arg to
	// box.Context. We register no providers for now (no ACME/Tailscale/Origin
	// CA in a desktop client), but the registry itself must be non-nil.
	return certificate.NewRegistry()
}
