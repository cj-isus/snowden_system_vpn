package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"snowden-system/backend/core"
)

// TelegramLogger sends periodic status + error reports to a Telegram bot AND
// runs an interactive admin panel via inline keyboard buttons.
type TelegramLogger struct {
	token    string
	chatID   string
	ownerID  string // optional: restrict commands to a specific user
	engine   *core.Engine
	adaptive *core.AdaptiveEngine
	manager  *core.Manager
	hostname string // machine label for "from whom" marking

	mu       sync.Mutex
	buffer   []string
	lastSend time.Time
	offset   int64 // getUpdates offset

	// Device registry — tracks which devices reported recently.
	// Each device (PC hostname, Android model) registers itself.
	devices map[string]*DeviceInfo
}

// DeviceInfo tracks one connected device.
type DeviceInfo struct {
	Name      string    // hostname or device model
	Platform  string    // "PC", "Android", "iOS"
	LastSeen  time.Time // last report time
	Status    string    // "connected", "disconnected", "error"
	IP        string    // external IP if known
}

const (
	tgCheckInterval = 1 * time.Minute  // how often we poll for errors
	tgMinReportGap  = 1 * time.Hour    // never report more often than this
	tgMaxBuffer     = 50
	tgCriticalCats  = "server_blocked|whitelist_mode|network_down"
)

// NewTelegramLogger creates a logger that posts to the given bot chat.
func NewTelegramLogger(token, chatID string, engine *core.Engine, adaptive *core.AdaptiveEngine) *TelegramLogger {
	host, _ := os.Hostname()
	if host == "" {
		host = "PC"
	}
	return &TelegramLogger{
		token:    token,
		chatID:   chatID,
		engine:   engine,
		adaptive: adaptive,
		hostname: host,
		devices:  make(map[string]*DeviceInfo),
	}
}

// SetManager wires the manager (for server list / restart in admin panel).
func (tl *TelegramLogger) SetManager(m *core.Manager) {
	tl.manager = m
}

// Start launches the background sender + command poller goroutines.
func (tl *TelegramLogger) Start(ctx context.Context) {
	go tl.loop(ctx)
	go tl.commandLoop(ctx)
}

// PushLog adds a log line to the buffer (called from logEmitter).
func (tl *TelegramLogger) PushLog(line string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.buffer = append(tl.buffer, line)
}

// ─── Periodic report sender ──────────────────────────────────────────────────
// Reports are sent ONLY when the tunnel has an error/unhealthy state, and never
// more often than once per hour. Normal "everything works" state does NOT
// generate a report — the user only hears from the bot when something is wrong.

func (tl *TelegramLogger) loop(ctx context.Context) {
	// Wait for the engine to start running so the proxy is available.
	// The startup message goes through the proxy (Telegram is blocked in RU).
	for i := 0; i < 30 && !tl.engine.Running(); i++ {
		time.Sleep(time.Second)
	}
	// Small extra delay for the proxy port to be ready
	time.Sleep(2 * time.Second)

	// Send a startup message (always — so the user knows it's alive)
	tl.send(fmt.Sprintf("🟢 *snowden.system* запущен на `%s`\nМолчу пока всё ок. Сообщу при ошибках (не чаще раза в час).", tl.hostname))
	ticker := time.NewTicker(tgCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			tl.send(fmt.Sprintf("🔴 *snowden.system* остановлен на `%s`", tl.hostname))
			return
		case <-ticker.C:
			tl.maybeReportError()
		}
	}
}

// maybeReportError checks the tunnel state and sends a report ONLY if:
//  1. there are errors/warnings in the recent log buffer, AND
//  2. the circuit breaker category is not "healthy", AND
//  3. at least tgMinReportGap (1h) has passed since the last report.
func (tl *TelegramLogger) maybeReportError() {
	diag := tl.adaptive.Diagnostics()

	// Is there anything worth reporting?
	hasErrors := diag.Category != "" && diag.Category != "healthy"
	tl.mu.Lock()
	recentErrors := 0
	for _, l := range tl.buffer {
		if containsAny(l, "[error] [adaptive] [diag] server_down tls_failure") {
			recentErrors++
		}
	}
	tl.mu.Unlock()

	if !hasErrors && recentErrors == 0 {
		return // everything is fine — stay silent
	}

	// Throttle: don't send more often than once per hour
	tl.mu.Lock()
	gap := time.Since(tl.lastSend)
	tl.mu.Unlock()
	if gap < tgMinReportGap {
		return // too soon since last report
	}

	tl.sendReport()
}

func (tl *TelegramLogger) sendReport() {
	tl.mu.Lock()
	tl.lastSend = time.Now()
	logs := tl.buffer
	tl.buffer = nil
	tl.mu.Unlock()

	diag := tl.adaptive.Diagnostics()
	st := tl.engine.State()

	msg := fmt.Sprintf("⚠️ *snowden.system* — ошибка на `%s`\n\n", tl.hostname)
	msg += fmt.Sprintf("⏱ %s\n", time.Now().Format("15:04:05"))
	msg += fmt.Sprintf("📡 Состояние: `%s`\n", st.String())
	msg += fmt.Sprintf("🔧 Circuit: `%s`\n", diag.State)
	if diag.Category != "healthy" && diag.Category != "" {
		msg += fmt.Sprintf("⚠️ Категория: `%s`\n", diag.Category)
		msg += fmt.Sprintf("📝 %s\n", diag.Explanation)
	}
	if diag.FailCount > 0 {
		msg += fmt.Sprintf("❌ Ошибок подряд: %d\n", diag.FailCount)
	}
	// Live traffic
	if tl.manager != nil {
		t := tl.manager.GetTraffic()
		if t.Uptime > 0 {
			msg += fmt.Sprintf("📊 Трафик: ↓%s ↑%s · uptime %ds\n",
				fmtBytes(t.DownloadTotal), fmtBytes(t.UploadTotal), t.Uptime)
		}
	}

	var important []string
	for _, l := range logs {
		if containsAny(l, "[error] [warn] [adaptive] [diag]") {
			important = append(important, l)
		}
	}
	if len(important) > 8 {
		important = important[len(important)-8:]
	}
	if len(important) > 0 {
		msg += "\n📋 *События:*\n"
		for _, l := range important {
			ll := l
			if len(ll) > 80 {
				ll = ll[:80] + "…"
			}
			msg += fmt.Sprintf("  `%s`\n", ll)
		}
	}

	// Inline keyboard for admin panel
	toggleText := "▶ Старт"
	if st == core.StateRunning {
		toggleText = "⏹ Стоп"
	}
	kb := tgInlineKeyboard{
		InlineKeyboard: [][]tgButton{
			{{Text: "📊 Статус", CallbackData: "status"}, {Text: "🌐 Серверы", CallbackData: "servers"}},
			{{Text: toggleText, CallbackData: "toggle"}, {Text: "🔄 Переподключить", CallbackData: "reconnect"}},
			{{Text: "📈 Трафик", CallbackData: "traffic"}, {Text: "🩺 Диагностика", CallbackData: "diag"}},
		},
	}
	tl.sendWithKeyboard(msg, kb)
}

// ─── Command poller (getUpdates) ─────────────────────────────────────────────

type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Text string `json:"text"`
	} `json:"message"`
	CallbackQuery *struct {
		ID      string `json:"id"`
		From    struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Message struct {
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
		Data string `json:"data"`
	} `json:"callback_query"`
}

func (tl *TelegramLogger) commandLoop(ctx context.Context) {
	// Wait a bit for sing-box to be ready before polling commands
	time.Sleep(5 * time.Second)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tl.pollUpdates()
		}
	}
}

func (tl *TelegramLogger) pollUpdates() {
	if tl.token == "" {
		return
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=5&offset=%d", tl.token, tl.offset)
	client := tl.makeClient()
	resp, err := client.Get(apiURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var result struct {
		OK     bool       `json:"ok"`
		Result []tgUpdate `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}
	for _, upd := range result.Result {
		tl.offset = upd.UpdateID + 1
		if upd.CallbackQuery != nil {
			tl.handleCallback(upd.CallbackQuery.ID, upd.CallbackQuery.Data, upd.CallbackQuery.From.ID, upd.CallbackQuery.From.FirstName)
			tl.answerCallback(upd.CallbackQuery.ID)
		}
		if upd.Message != nil {
			tl.handleCommand(upd.Message.Text, upd.Message.From.ID, upd.Message.From.FirstName, upd.Message.Chat.ID)
		}
	}
}

// Admin Telegram ID — loaded from environment variable SNOWDEN_TG_CHAT_ID.
var adminUserID = getAdminUserID()

func getAdminUserID() int64 {
	idStr := os.Getenv("SNOWDEN_TG_ADMIN_ID")
	if idStr == "" {
		idStr = os.Getenv("SNOWDEN_TG_CHAT_ID")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// File download URLs (served from VPS nginx:8090)
var (
	fileBaseURL = os.Getenv("SNOWDEN_FILE_URL")
	filePC      = fileBaseURL + "/snowden-portable.zip"
	fileAPK     = fileBaseURL + "/snowden-android.apk"
	fileIOS     = fileBaseURL + "/snowden-ios-config.json"
)

func (tl *TelegramLogger) handleCommand(text string, userID int64, fromName string, chatID int64) {
	isAdmin := userID == adminUserID

	switch text {
	// === Команды для ВСЕХ ===
	case "/start", "/help":
		if isAdmin {
			tl.sendToChat(chatID, fmt.Sprintf("🛡 *snowden.system* — админ-панель\nУстройство: `%s`\n\n*Команды:*\n/status — статус VPN\n/servers — серверы\n/traffic — трафик\n/reconnect — переподключить\n/toggle — вкл/выкл\n/logs — отправить логи\n/devices — список устройств\n\n*Файлы:*\n/pc — скачать ПК\n/apk — скачать Android\n/ios — конфиг iOS", tl.hostname))
		} else {
			tl.sendToChat(chatID, "🌐 *snowden.system*\n\nСкачайте приложение:\n\n/pc — Windows ПК\n/apk — Android\n/ios — конфиг для iPhone\n\nСайт: `snowden-system.pages.dev`")
		}

	case "/logs":
		if isAdmin {
			tl.sendLogs(chatID)
		} else {
			tl.sendToChat(chatID, "⛔ Только для администратора")
		}

	case "/devices":
		if isAdmin {
			tl.sendDevicesList(chatID)
		} else {
			tl.sendToChat(chatID, "⛔ Только для администратора")
		}

	case "/pc":
		tl.sendFile(chatID, "💻 Windows ПК (v1.1.7)", filePC)
	case "/apk":
		tl.sendFile(chatID, "📱 Android (v1.1.7)", fileAPK)
	case "/ios":
		tl.sendFile(chatID, "🍎 Конфиг для sing-box (iOS)", fileIOS)

	// === Только АДМИН ===
	case "/status":
		if isAdmin { tl.sendStatus() } else { tl.sendToChat(chatID, "⛔ Команда только для администратора") }
	case "/servers":
		if isAdmin { tl.sendServers() } else { tl.sendToChat(chatID, "⛔ Только для администратора") }
	case "/traffic":
		if isAdmin { tl.sendTraffic() } else { tl.sendToChat(chatID, "⛔ Только для администратора") }
	case "/reconnect", "/restart":
		if isAdmin {
			go func() { defer func() { recover() }(); tl.doReconnect() }()
		} else {
			tl.sendToChat(chatID, "⛔ Только для администратора")
		}
	case "/toggle":
		if isAdmin {
			go func() { defer func() { recover() }(); tl.doToggle() }()
		} else {
			tl.sendToChat(chatID, "⛔ Только для администратора")
		}
	default:
		if text != "" && !isAdmin {
			tl.sendToChat(chatID, "Напишите /help — список команд")
		}
	}
}

func (tl *TelegramLogger) handleCallback(cbID, data string, userID int64, fromName string) {
	isAdmin := userID == adminUserID
	chatID, _ := strconv.ParseInt(tl.chatID, 10, 64)

	switch data {
	// Файловые команды — для всех
	case "pc":
		tl.sendFile(chatID, "💻 Windows ПК", filePC)
	case "apk":
		tl.sendFile(chatID, "📱 Android APK", fileAPK)
	case "ios":
		tl.sendFile(chatID, "🍎 iOS конфиг", fileIOS)
	// Админские — только админ
	case "status":
		if isAdmin { tl.sendStatus() }
	case "servers":
		if isAdmin { tl.sendServers() }
	case "toggle":
		if isAdmin { go func() { defer func() { recover() }(); tl.doToggle() }() }
	case "reconnect":
		if isAdmin { go func() { defer func() { recover() }(); tl.doReconnect() }() }
	case "traffic":
		if isAdmin { tl.sendTraffic() }
	case "diag":
		if isAdmin { tl.sendDiag() }
	case "logs":
		if isAdmin { chatID, _ := strconv.ParseInt(tl.chatID, 10, 64); tl.sendLogs(chatID) }
	case "devices":
		if isAdmin { chatID, _ := strconv.ParseInt(tl.chatID, 10, 64); tl.sendDevicesList(chatID) }
	}
}

func (tl *TelegramLogger) sendStatus() {
	diag := tl.adaptive.Diagnostics()
	st := tl.engine.State()
	emoji := "🔴"
	if st == core.StateRunning {
		emoji = "🟢"
	}
	msg := fmt.Sprintf("%s *Статус* `%s`\n", emoji, tl.hostname)
	msg += fmt.Sprintf("Состояние: `%s`\n", st.String())
	msg += fmt.Sprintf("Circuit: `%s`\n", diag.State)
	if diag.Category != "" && diag.Category != "healthy" {
		msg += fmt.Sprintf("⚠️ %s\n", diag.Explanation)
	}
	tl.sendWithKeyboard(msg, tl.adminKeyboard())
}

func (tl *TelegramLogger) sendServers() {
	if tl.manager == nil {
		tl.send("Серверы недоступны")
		return
	}
	servers := tl.manager.GetServers()
	if len(servers) == 0 {
		tl.send("Серверы не загружены. Запустите VPN.")
		return
	}
	msg := fmt.Sprintf("🌐 *Серверы* `%s`\n\n", tl.hostname)
	for _, s := range servers {
		marker := "⚪"
		if s.Active {
			marker = "🟢"
		}
		ping := "—"
		if s.Ping >= 0 {
			ping = fmt.Sprintf("%dms", s.Ping)
		}
		msg += fmt.Sprintf("%s `%s` · %s\n   %s:%d · %s\n", marker, s.Name, s.Protocol, s.Server, s.Port, ping)
	}
	tl.send(msg)
}

func (tl *TelegramLogger) sendTraffic() {
	if tl.manager == nil {
		tl.send("Трафик недоступен")
		return
	}
	t := tl.manager.GetTraffic()
	msg := fmt.Sprintf("📈 *Трафик* `%s`\n\n", tl.hostname)
	msg += fmt.Sprintf("↓ Скачано: %s (%s/s)\n", fmtBytes(t.DownloadTotal), fmtBytes(t.DownloadSpeed))
	msg += fmt.Sprintf("↑ Отправлено: %s (%s/s)\n", fmtBytes(t.UploadTotal), fmtBytes(t.UploadSpeed))
	if t.Uptime > 0 {
		h := t.Uptime / 3600
		m := (t.Uptime % 3600) / 60
		msg += fmt.Sprintf("⏱ Uptime: %dч %dм\n", h, m)
	}
	tl.send(msg)
}

func (tl *TelegramLogger) sendDiag() {
	diag := tl.adaptive.Diagnostics()
	msg := fmt.Sprintf("🩺 *Диагностика* `%s`\n\n", tl.hostname)
	msg += fmt.Sprintf("State: `%s`\n", diag.State)
	msg += fmt.Sprintf("Category: `%s`\n", diag.Category)
	msg += fmt.Sprintf("Fail count: %d\n", diag.FailCount)
	msg += fmt.Sprintf("OK count: %d\n", diag.OkCount)
	if diag.LastError != "" {
		le := diag.LastError
		if len(le) > 200 {
			le = le[:200] + "…"
		}
		msg += fmt.Sprintf("\n```\n%s\n```", le)
	}
	tl.send(msg)
}

func (tl *TelegramLogger) doToggle() {
	if tl.engine.Running() {
		// Stop
		if tl.manager != nil {
			tl.manager.StopVPN()
		}
		tl.send(fmt.Sprintf("⏹ VPN остановлен на `%s`", tl.hostname))
	} else {
		// Start — reload last config
		if tl.manager != nil {
			cfg := tl.manager.ActiveConfigJSON()
			if len(cfg) > 0 {
				if err := tl.manager.StartVPN("vps-reality", cfg); err != nil {
					tl.send(fmt.Sprintf("❌ Ошибка старта: `%s`", err.Error()))
					return
				}
				tl.send(fmt.Sprintf("▶ VPN запущен на `%s`", tl.hostname))
			} else {
				tl.send("❌ Нет сохранённого конфига. Запустите VPN из приложения.")
			}
		}
	}
}

func (tl *TelegramLogger) doReconnect() {
	if tl.manager == nil {
		tl.send("Менеджер недоступен")
		return
	}
	cfg := tl.manager.ActiveConfigJSON()
	if len(cfg) == 0 {
		tl.send("❌ Нет активного конфига")
		return
	}
	tl.send(fmt.Sprintf("🔄 Переподключение на `%s`…", tl.hostname))
	if err := tl.manager.ReloadVPN("vps-reality", cfg); err != nil {
		tl.send(fmt.Sprintf("❌ Ошибка: `%s`", err.Error()))
		return
	}
	tl.send(fmt.Sprintf("✅ Переподключено на `%s`", tl.hostname))
}

// sendLogs collects recent log buffer and sends it to Telegram.
func (tl *TelegramLogger) sendLogs(chatID int64) {
	tl.mu.Lock()
	logs := make([]string, len(tl.buffer))
	copy(logs, tl.buffer)
	tl.mu.Unlock()

	if len(logs) == 0 {
		tl.sendToChat(chatID, "📋 Лог пуст")
		return
	}

	// Take last 50 lines
	if len(logs) > 50 {
		logs = logs[len(logs)-50:]
	}

	// Filter: only errors/warnings/important
	var important []string
	for _, l := range logs {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "error") || strings.Contains(lower, "warn") ||
			strings.Contains(lower, "[!") || strings.Contains(lower, "fail") ||
			strings.Contains(lower, "⚠️") || strings.Contains(lower, "🚨") ||
			strings.Contains(lower, "urltest") || strings.Contains(lower, "available") ||
			strings.Contains(lower, "hysteria") || strings.Contains(lower, "tls") {
			important = append(important, l)
		}
	}

	if len(important) == 0 {
		important = logs // show all if no important
	}

	text := fmt.Sprintf("📋 *Логи %s* (последние %d):\n\n```\n", tl.hostname, len(important))
	for _, l := range important {
		ll := l
		if len(ll) > 100 {
			ll = ll[:100] + "…"
		}
		text += ll + "\n"
	}
	text += "```"
	tl.sendToChat(chatID, text)
}

// sendDevicesList shows all registered devices.
func (tl *TelegramLogger) sendDevicesList(chatID int64) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if len(tl.devices) == 0 {
		// Register self
		tl.sendToChat(chatID, fmt.Sprintf("📱 *Устройства:* только `%s` (ПК)\n\nAndroid-устройства регистрируются автоматически при подключении.", tl.hostname))
		return
	}

	msg := "📱 *Подключённые устройства:*\n\n"
	for name, dev := range tl.devices {
		age := time.Since(dev.LastSeen)
		ageStr := "только что"
		if age > time.Minute {
			ageStr = fmt.Sprintf("%d мин назад", int(age.Minutes()))
		}
		statusEmoji := "🟢"
		if dev.Status == "disconnected" {
			statusEmoji = "🔴"
		} else if dev.Status == "error" {
			statusEmoji = "❌"
		}
		msg += fmt.Sprintf("%s `%s` (%s)\n   %s · %s\n", statusEmoji, name, dev.Platform, dev.Status, ageStr)
	}
	tl.sendToChat(chatID, msg)
}

// RegisterDevice adds or updates a device in the registry (called from Android reports).
func (tl *TelegramLogger) RegisterDevice(name, platform, status string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if tl.devices == nil {
		tl.devices = make(map[string]*DeviceInfo)
	}
	tl.devices[name] = &DeviceInfo{
		Name:     name,
		Platform: platform,
		LastSeen: time.Now(),
		Status:   status,
	}
}

func (tl *TelegramLogger) adminKeyboard() tgInlineKeyboard {
	st := tl.engine.State()
	toggleText := "▶ Старт"
	if st == core.StateRunning {
		toggleText = "⏹ Стоп"
	}
	return tgInlineKeyboard{
		InlineKeyboard: [][]tgButton{
			{{Text: "📊 Статус", CallbackData: "status"}, {Text: "🌐 Серверы", CallbackData: "servers"}},
			{{Text: toggleText, CallbackData: "toggle"}, {Text: "🔄 Переподключить", CallbackData: "reconnect"}},
			{{Text: "📈 Трафик", CallbackData: "traffic"}, {Text: "🩺 Диагностика", CallbackData: "diag"}},
			{{Text: "📋 Логи", CallbackData: "logs"}, {Text: "📱 Устройства", CallbackData: "devices"}},
		},
	}
}

// ─── HTTP helpers ────────────────────────────────────────────────────────────

type tgButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// sendToChat sends a message to a specific chat ID (for non-admin users).
func (tl *TelegramLogger) sendToChat(chatID int64, text string) {
	tl.sendWithKeyboardToChat(chatID, text, tgInlineKeyboard{})
}

// sendFile downloads a file from the given URL and sends it to the chat.
func (tl *TelegramLogger) sendFile(chatID int64, caption, fileURL string) {
	go func() {
		defer func() { recover() }()
		client := tl.makeClient()
		resp, err := client.Get(fileURL)
		if err != nil {
			tl.sendToChat(chatID, "❌ Не удалось скачать файл")
			return
		}
		defer resp.Body.Close()

		// Send as document
		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument?chat_id=%d&caption=%s",
			tl.token, chatID, caption)
		_, err = client.Post(apiURL, "application/octet-stream", resp.Body)
		if err != nil {
			// Fallback: send direct link
			tl.sendToChat(chatID, fmt.Sprintf("⬇️ Скачайте напрямую:\n%s", fileURL))
		}
	}()
}

// sendWithKeyboardToChat sends a message with optional keyboard to a specific chat.
func (tl *TelegramLogger) sendWithKeyboardToChat(chatID int64, text string, kb tgInlineKeyboard) {
	if tl.token == "" {
		return
	}
	if len(text) > 4000 {
		text = text[:4000] + "…"
	}
	payload := map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	// For non-admin users: show file download buttons
	if chatID != adminUserID && len(kb.InlineKeyboard) == 0 {
		kb = tgInlineKeyboard{
			InlineKeyboard: [][]tgButton{
				{{Text: "💻 ПК", CallbackData: "pc"}, {Text: "📱 Android", CallbackData: "apk"}, {Text: "🍎 iOS", CallbackData: "ios"}},
			},
		}
	}
	if len(kb.InlineKeyboard) > 0 {
		payload["reply_markup"] = kb
	}
	body, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tl.token)
	client := tl.makeClient()
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

type tgInlineKeyboard struct {
	InlineKeyboard [][]tgButton `json:"inline_keyboard"`
}

func (tl *TelegramLogger) makeClient() *http.Client {
	proxyURL, _ := url.Parse("http://127.0.0.1:20808")
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	// CRITICAL: Use a custom resolver that forces IPv4-only resolution.
	// Windows DNS returns IPv6 (AAAA) records for api.telegram.org, but our VPS
	// has no IPv6 route → connection hangs with "no such host" / timeout.
	// This resolver filters out AAAA records, keeping only A (IPv4).
	ipv4Resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Use Google DNS directly over UDP (avoids system resolver IPv6 leak)
			return dialer.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}

	if tl.engine.Running() {
		// Route through the local sing-box proxy.
		// The proxy handles DNS + CONNECT to Telegram.
		// DialContext forces IPv4 for the connection TO the proxy (127.0.0.1).
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", addr)
			},
		}
		return &http.Client{Timeout: 20 * time.Second, Transport: transport}
	}

	// Direct (no proxy) — force IPv4 with custom resolver
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			// Resolve via IPv4-only resolver
			ips, err := ipv4Resolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var ipv4 string
			for _, ip := range ips {
				if ip.IP.To4() != nil {
					ipv4 = ip.IP.String()
					break
				}
			}
			if ipv4 == "" {
				return nil, fmt.Errorf("no IPv4 address for %s", host)
			}
			return dialer.DialContext(ctx, "tcp4", net.JoinHostPort(ipv4, port))
		},
	}
	return &http.Client{Timeout: 20 * time.Second, Transport: transport}
}

func (tl *TelegramLogger) send(text string) {
	tl.sendWithKeyboard(text, tgInlineKeyboard{})
}

func (tl *TelegramLogger) sendWithKeyboard(text string, kb tgInlineKeyboard) {
	if tl.token == "" || tl.chatID == "" {
		return
	}
	if len(text) > 4000 {
		text = text[:4000] + "…"
	}
	payload := map[string]any{
		"chat_id":    tl.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	if len(kb.InlineKeyboard) > 0 {
		payload["reply_markup"] = kb
	}
	body, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tl.token)

	client := tl.makeClient()
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		// Retry direct (no proxy) — still IPv4
		directClient := &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp4", addr)
				},
			},
		}
		resp, err = directClient.Post(apiURL, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("[telegram] send failed: %v", err)
			return
		}
	}
	resp.Body.Close()
}

func (tl *TelegramLogger) answerCallback(cbID string) {
	if tl.token == "" || cbID == "" {
		return
	}
	payload, _ := json.Marshal(map[string]string{"callback_query_id": cbID})
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", tl.token)
	client := tl.makeClient()
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(payload))
	if err == nil {
		resp.Body.Close()
	}
}

func containsAny(s, list string) bool {
	start := 0
	for i := 0; i <= len(list); i++ {
		if i == len(list) || list[i] == '|' {
			sub := list[start:i]
			if sub != "" && len(s) >= len(sub) {
				for j := 0; j <= len(s)-len(sub); j++ {
					match := true
					for k := 0; k < len(sub); k++ {
						if s[j+k] != sub[k] {
							match = false
							break
						}
					}
					if match {
						return true
					}
					_ = sub
				}
			}
			start = i + 1
		}
	}
	return false
}

func fmtBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1048576 {
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	}
	if b < 1073741824 {
		return fmt.Sprintf("%.2fMB", float64(b)/1048576)
	}
	return fmt.Sprintf("%.2fGB", float64(b)/1073741824)
}
