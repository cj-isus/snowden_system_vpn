package core

import (
	"strings"
	"sync"
	"time"
)

// ErrorCategory is a high-level classification of a tunnel problem.
type ErrorCategory int

const (
	CatHealthy ErrorCategory = iota
	CatNetworkDown    // local internet is dead
	CatServerDown     // VPS unreachable
	CatDNSFailure     // DNS resolution failing
	CatTLSFailure     // TLS handshake failed
	CatServerBlocked  // DPI/TSPU blocking
	CatWhitelistMode  // whitelist (БС) detected
	CatDegraded       // works but slow
	CatUnknown        // unrecognised error
)

// String returns a human-readable name for the UI.
func (c ErrorCategory) String() string {
	switch c {
	case CatHealthy:
		return "healthy"
	case CatNetworkDown:
		return "network_down"
	case CatServerDown:
		return "server_down"
	case CatDNSFailure:
		return "dns_failure"
	case CatTLSFailure:
		return "tls_failure"
	case CatServerBlocked:
		return "server_blocked"
	case CatWhitelistMode:
		return "whitelist_mode"
	case CatDegraded:
		return "degraded"
	default:
		return "unknown"
	}
}

// HumanExplain returns a user-friendly description in Russian.
func (c ErrorCategory) HumanExplain() string {
	switch c {
	case CatHealthy:
		return "Всё работает нормально"
	case CatNetworkDown:
		return "Нет интернета. Проверьте подключение."
	case CatServerDown:
		return "Сервер VPN не отвечает. Возможно, упал или заблокирован."
	case CatDNSFailure:
		return "Ошибка DNS. Не удаётся разрешить доменные имена."
	case CatTLSFailure:
		return "TLS-рукопожатие не удалось. Провайдер может блокировать."
	case CatServerBlocked:
		return "ТСПУ/DPI заблокировал соединение."
	case CatWhitelistMode:
		return "Обнаружен режим белых списков (БС). Обход невозможен."
	case CatDegraded:
		return "Туннель работает медленно."
	default:
		return "Неизвестная ошибка."
	}
}

// DiagEvent represents a single classified diagnostic event.
type DiagEvent struct {
	Timestamp time.Time
	Category  ErrorCategory
	RawLine   string
}

// ErrorClassifier collects sing-box log lines, classifies them into
// ErrorCategory values, and maintains a rolling history. The AdaptiveEngine
// reads the latest category to drive its circuit-breaker decisions.
type ErrorClassifier struct {
	mu          sync.Mutex
	current     ErrorCategory
	lastError   string
	events      []DiagEvent
	maxEvents   int
}

// NewErrorClassifier creates a classifier with a rolling buffer of maxEvents.
func NewErrorClassifier(maxEvents int) *ErrorClassifier {
	return &ErrorClassifier{
		current:   CatHealthy,
		maxEvents: maxEvents,
	}
}

// OnLog is called for every sing-box log line (via PlatformWriter).
// It classifies the line and updates the current category if it contains
// an error pattern. Only [error] and [warn] level lines are classified —
// debug/info/trace lines are ignored to avoid false positives.
func (ec *ErrorClassifier) OnLog(line string) {
	// Quick filter: only classify error/warn lines.
	if !strings.Contains(line, "[error]") && !strings.Contains(line, "[warn]") {
		return
	}
	cat := classify(line)
	if cat == CatHealthy || cat == CatUnknown {
		return // not a recognised error, ignore
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.current = cat
	ec.lastError = line
	ec.events = append(ec.events, DiagEvent{
		Timestamp: time.Now(),
		Category:  cat,
		RawLine:   line,
	})
	if len(ec.events) > ec.maxEvents {
		ec.events = ec.events[len(ec.events)-ec.maxEvents:]
	}
}

// Current returns the latest classified error category.
func (ec *ErrorClassifier) Current() ErrorCategory {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.current
}

// LastError returns the raw log line of the most recent error.
func (ec *ErrorClassifier) LastError() string {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.lastError
}

// Reset sets the category back to Healthy (called after successful recovery).
func (ec *ErrorClassifier) Reset() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.current = CatHealthy
}

// Events returns a copy of the recent diagnostic events (for the UI).
func (ec *ErrorClassifier) Events() []DiagEvent {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	out := make([]DiagEvent, len(ec.events))
	copy(out, ec.events)
	return out
}

// classify examines a single log line and returns its category.
// Patterns are ordered from most specific to least.
func classify(line string) ErrorCategory {
	lower := strings.ToLower(line)

	// Whitelist mode: all blocked sites fail + RU net works.
	// This is detected by the AdaptiveEngine via correlation, not by a single
	// log line. But a strong signal is many different hosts timing out.
	if strings.Contains(lower, "whitelist") || strings.Contains(lower, "белый список") {
		return CatWhitelistMode
	}

	// REALITY blocked
	if strings.Contains(lower, "reality: processed invalid") ||
		strings.Contains(lower, "reality: failed") {
		return CatServerBlocked
	}

	// TLS handshake failures
	if strings.Contains(lower, "tls handshake") &&
		(strings.Contains(lower, "timeout") ||
			strings.Contains(lower, "failed") ||
			strings.Contains(lower, "eof") ||
			strings.Contains(lower, "context deadline exceeded")) {
		return CatTLSFailure
	}

	// DNS failures — only match actual error lines, not normal DNS debug logs
	if strings.Contains(lower, "lookup failed") ||
		strings.Contains(lower, "no such host") ||
		(strings.Contains(lower, "dns:") && strings.Contains(lower, "error")) {
		return CatDNSFailure
	}

	// Server unreachable — dial errors
	if strings.Contains(lower, "dial tcp") &&
		(strings.Contains(lower, "i/o timeout") ||
			strings.Contains(lower, "connection refused") ||
			strings.Contains(lower, "no route to host")) {
		return CatServerDown
	}

	// Connection reset by DPI/TSPU
	if strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "wsarecv: an existing connection was forcibly closed") {
		// This alone is ambiguous — could be normal (app closing conn).
		// Only flag if it's on an outbound connection.
		if strings.Contains(lower, "outbound") || strings.Contains(lower, "vless") {
			return CatServerBlocked
		}
		return CatUnknown
	}

	// Context deadline = general timeout on outbound
	if strings.Contains(lower, "context deadline exceeded") &&
		strings.Contains(lower, "outbound") {
		return CatServerDown
	}

	// Empty reply / EOF on outbound
	if strings.Contains(lower, "eof") && strings.Contains(lower, "outbound") {
		return CatServerDown
	}

	return CatUnknown
}

// ClassifyProbeError classifies an error from the health-check probe (not a
// sing-box log line). This lets the circuit breaker distinguish "local net
// is dead" from "server is unreachable."
func ClassifyProbeError(err error) ErrorCategory {
	if err == nil {
		return CatHealthy
	}
	msg := strings.ToLower(err.Error())

	// Proxy connection refused = local proxy port not listening = engine down
	if strings.Contains(msg, "connection refused") &&
		strings.Contains(msg, "127.0.0.1") {
		return CatNetworkDown // likely the engine process itself is dead
	}

	// Timeout through proxy = tunnel not delivering traffic
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		// Could be server or network; the caller should disambiguate with
		// a direct (non-proxy) probe.
		return CatServerDown
	}

	// Connection reset through proxy
	if strings.Contains(msg, "connection reset") || strings.Contains(msg, "forcibly closed") {
		return CatServerBlocked
	}

	return CatUnknown
}
