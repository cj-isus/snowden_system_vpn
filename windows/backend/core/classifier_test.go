package core

import (
	"fmt"
	"strings"
	"testing"
)

// ─── classify() unit tests ──────────────────────────────────────────────────

func TestClassifyWhitelistMode(t *testing.T) {
	lines := []string{
		"[error] whitelist mode detected, skipping direct connection",
		"[error] белый список активен, прямое подключение невозможно",
		"[warn] fallback to whitelist mode for domain example.com",
	}
	for _, line := range lines {
		if got := classify(line); got != CatWhitelistMode {
			t.Errorf("classify(%q) = %v, want CatWhitelistMode", line, got)
		}
	}
}

func TestClassifyRealityBlocked(t *testing.T) {
	lines := []string{
		"[error] reality: processed invalid server name",
		"[error] reality: failed to establish connection to server",
		"[warn] reality: processed invalid certificate from 1.2.3.4",
	}
	for _, line := range lines {
		if got := classify(line); got != CatServerBlocked {
			t.Errorf("classify(%q) = %v, want CatServerBlocked", line, got)
		}
	}
}

func TestClassifyTLSFailure(t *testing.T) {
	lines := []string{
		"[error] tls handshake timeout to 1.2.3.4:443",
		"[error] tls handshake failed with server",
		"[warn] tls handshake eof from vless-outbound",
		"[error] outbound/vless: tls handshake context deadline exceeded",
	}
	for _, line := range lines {
		if got := classify(line); got != CatTLSFailure {
			t.Errorf("classify(%q) = %v, want CatTLSFailure", line, got)
		}
	}
}

func TestClassifyTLSHandshakeWithoutFailureKeywordIsUnknown(t *testing.T) {
	// A TLS handshake log that doesn't contain timeout/failed/eof/deadline
	// should NOT be classified as TLS failure by classify() itself.
	// (OnLog would also skip it since it's [info], not [error]/[warn].)
	line := "[info] tls handshake completed successfully to 1.2.3.4:443"
	if got := classify(line); got == CatTLSFailure {
		t.Errorf("classify(%q) should not be CatTLSFailure, got %v", line, got)
	}
}

func TestClassifyDNSFailure(t *testing.T) {
	lines := []string{
		"[error] lookup failed for example.com: no such host",
		"[error] dns: error resolving api.openai.com",
		"[warn] no such host for google.com",
	}
	for _, line := range lines {
		if got := classify(line); got != CatDNSFailure {
			t.Errorf("classify(%q) = %v, want CatDNSFailure", line, got)
		}
	}
}

func TestClassifyServerDown(t *testing.T) {
	lines := []string{
		"[error] dial tcp 1.2.3.4:443: i/o timeout",
		"[error] dial tcp 1.2.3.4:443: connection refused",
		"[error] dial tcp 1.2.3.4:443: no route to host",
		"[error] outbound/vless: context deadline exceeded",
		"[error] outbound/vless: eof",
	}
	for _, line := range lines {
		if got := classify(line); got != CatServerDown {
			t.Errorf("classify(%q) = %v, want CatServerDown", line, got)
		}
	}
}

func TestClassifyServerBlockedViaConnectionReset(t *testing.T) {
	lines := []string{
		"[error] outbound/vless: connection reset by peer",
		"[error] vless outbound: connection reset",
	}
	for _, line := range lines {
		if got := classify(line); got != CatServerBlocked {
			t.Errorf("classify(%q) = %v, want CatServerBlocked", line, got)
		}
	}
}

func TestClassifyWsarecvRequiresOutboundContext(t *testing.T) {
	// wsarecv without "outbound" or "vless" context → CatUnknown (ambiguous).
	line := "[error] wsarecv: an existing connection was forcibly closed by the remote host"
	if got := classify(line); got != CatUnknown {
		t.Errorf("classify(%q) = %v, want CatUnknown (no outbound context)", line, got)
	}

	// wsarecv with outbound context → CatServerBlocked.
	line2 := "[error] outbound/wsarecv: an existing connection was forcibly closed"
	if got := classify(line2); got != CatServerBlocked {
		t.Errorf("classify(%q) = %v, want CatServerBlocked", line2, got)
	}
}

func TestClassifyConnectionResetWithoutOutboundIsUnknown(t *testing.T) {
	line := "[warn] inbound/mixed: connection reset by peer"
	if got := classify(line); got != CatUnknown {
		t.Errorf("classify(%q) = %v, want CatUnknown", line, got)
	}
}

func TestClassifyUnknownLines(t *testing.T) {
	lines := []string{
		"[info] sing-box started successfully",
		"[debug] outbound/vless: connected to 1.2.3.4:443",
		"[trace] dns: resolved example.com to 93.184.216.34",
		"[info] mixed inbound listening on 127.0.0.1:20808",
		"[warn] something completely unrelated",
	}
	for _, line := range lines {
		if got := classify(line); got != CatUnknown {
			t.Errorf("classify(%q) = %v, want CatUnknown", line, got)
		}
	}
}

// ─── ClassifyProbeError tests ──────────────────────────────────────────────

func TestClassifyProbeErrorNil(t *testing.T) {
	if got := ClassifyProbeError(nil); got != CatHealthy {
		t.Errorf("ClassifyProbeError(nil) = %v, want CatHealthy", got)
	}
}

func TestClassifyProbeErrorLocalProxyDown(t *testing.T) {
	err := fmt.Errorf("dial tcp 127.0.0.1:20808: connection refused")
	if got := ClassifyProbeError(err); got != CatNetworkDown {
		t.Errorf("ClassifyProbeError(local refused) = %v, want CatNetworkDown", got)
	}
}

func TestClassifyProbeErrorTimeout(t *testing.T) {
	err := fmt.Errorf("Get \"http://www.gstatic.com/generate_204\": context deadline exceeded")
	if got := ClassifyProbeError(err); got != CatServerDown {
		t.Errorf("ClassifyProbeError(timeout) = %v, want CatServerDown", got)
	}
}

func TestClassifyProbeErrorConnectionReset(t *testing.T) {
	err := fmt.Errorf("read tcp: connection reset by peer")
	if got := ClassifyProbeError(err); got != CatServerBlocked {
		t.Errorf("ClassifyProbeError(reset) = %v, want CatServerBlocked", got)
	}
}

func TestClassifyProbeErrorForciblyClosed(t *testing.T) {
	err := fmt.Errorf("wsarecv: an existing connection was forcibly closed")
	if got := ClassifyProbeError(err); got != CatServerBlocked {
		t.Errorf("ClassifyProbeError(forcibly closed) = %v, want CatServerBlocked", got)
	}
}

func TestClassifyProbeErrorUnknown(t *testing.T) {
	err := fmt.Errorf("some random error that doesn't match any pattern")
	if got := ClassifyProbeError(err); got != CatUnknown {
		t.Errorf("ClassifyProbeError(unknown) = %v, want CatUnknown", got)
	}
}

// ─── ErrorClassifier (OnLog + events buffer) ────────────────────────────────

func TestClassifierOnLogIgnoresInfoLines(t *testing.T) {
	ec := NewErrorClassifier(10)
	ec.OnLog("[info] sing-box started successfully")
	ec.OnLog("[debug] outbound connected")
	ec.OnLog("[trace] dns resolved")

	if ec.Current() != CatHealthy {
		t.Errorf("Current() = %v after info/debug/trace, want CatHealthy", ec.Current())
	}
	if len(ec.Events()) != 0 {
		t.Errorf("Events() len = %d, want 0", len(ec.Events()))
	}
}

func TestClassifierOnLogClassifiesErrorLines(t *testing.T) {
	ec := NewErrorClassifier(10)
	ec.OnLog("[error] dial tcp 1.2.3.4:443: i/o timeout")

	if ec.Current() != CatServerDown {
		t.Errorf("Current() = %v, want CatServerDown", ec.Current())
	}
	events := ec.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	if events[0].Category != CatServerDown {
		t.Errorf("events[0].Category = %v, want CatServerDown", events[0].Category)
	}
	if !strings.Contains(events[0].RawLine, "i/o timeout") {
		t.Errorf("events[0].RawLine should contain original line, got %q", events[0].RawLine)
	}
}

func TestClassifierOnLogIgnoresHealthyAndUnknown(t *testing.T) {
	ec := NewErrorClassifier(10)
	ec.OnLog("[info] everything is fine")
	ec.OnLog("[warn] something weird happened")
	ec.OnLog("[error] just a debug leftover")

	if ec.Current() != CatHealthy {
		t.Errorf("Current() = %v, want CatHealthy (no recognised error)", ec.Current())
	}
}

func TestClassifierEventsRollingBuffer(t *testing.T) {
	maxEvents := 5
	ec := NewErrorClassifier(maxEvents)

	// Feed more errors than the buffer can hold.
	// Each line matches "dial tcp" + "i/o timeout" → CatServerDown → recorded.
	for i := 0; i < 20; i++ {
		ec.OnLog(fmt.Sprintf("[error] dial tcp 1.2.3.4:%d: i/o timeout", 1000+i))
	}

	events := ec.Events()
	if len(events) != maxEvents {
		t.Fatalf("Events() len = %d, want %d (rolling buffer)", len(events), maxEvents)
	}

	// Verify the last event is the most recent one (port = 1000+19 = 1019).
	wantPort := fmt.Sprintf("%d", 1000+19)
	if !strings.Contains(events[len(events)-1].RawLine, wantPort) {
		t.Errorf("last event should contain port %s, got %q", wantPort, events[len(events)-1].RawLine)
	}
}

func TestClassifierReset(t *testing.T) {
	ec := NewErrorClassifier(10)
	ec.OnLog("[error] dial tcp 1.2.3.4:443: connection refused")
	if ec.Current() != CatServerDown {
		t.Fatalf("pre-reset Current() = %v, want CatServerDown", ec.Current())
	}

	ec.Reset()
	if ec.Current() != CatHealthy {
		t.Errorf("post-reset Current() = %v, want CatHealthy", ec.Current())
	}
}

func TestClassifierLastError(t *testing.T) {
	ec := NewErrorClassifier(10)
	ec.OnLog("[error] dns: error resolving example.com")
	if ec.LastError() != "[error] dns: error resolving example.com" {
		t.Errorf("LastError() = %q, want the error line", ec.LastError())
	}

	ec.OnLog("[error] tls handshake timeout")
	if ec.LastError() != "[error] tls handshake timeout" {
		t.Errorf("LastError() after second error = %q", ec.LastError())
	}
}

func TestClassifierMultipleCategoriesTrackLatest(t *testing.T) {
	ec := NewErrorClassifier(10)

	ec.OnLog("[error] dial tcp 1.2.3.4:443: i/o timeout")
	if ec.Current() != CatServerDown {
		t.Errorf("after dial timeout: Current() = %v, want CatServerDown", ec.Current())
	}

	ec.OnLog("[error] tls handshake failed")
	if ec.Current() != CatTLSFailure {
		t.Errorf("after TLS fail: Current() = %v, want CatTLSFailure", ec.Current())
	}

	ec.OnLog("[error] lookup failed for api.openai.com")
	if ec.Current() != CatDNSFailure {
		t.Errorf("after DNS fail: Current() = %v, want CatDNSFailure", ec.Current())
	}
}

// ─── ErrorCategory String/HumanExplain exhaustiveness ──────────────────────

func TestErrorCategoryStringExhaustive(t *testing.T) {
	categories := []ErrorCategory{
		CatHealthy, CatNetworkDown, CatServerDown, CatDNSFailure,
		CatTLSFailure, CatServerBlocked, CatWhitelistMode, CatDegraded, CatUnknown,
	}
	seen := make(map[string]bool)
	for _, c := range categories {
		s := c.String()
		if s == "" {
			t.Errorf("String() for category %d is empty", c)
		}
		if seen[s] {
			t.Errorf("String() for category %d duplicates %q", c, s)
		}
		seen[s] = true

		h := c.HumanExplain()
		if h == "" {
			t.Errorf("HumanExplain() for category %d is empty", c)
		}
	}
}
