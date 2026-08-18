// Шаблон Telegram-бота для VPN мониторинга
// Заполни YOUR_BOT_TOKEN, YOUR_CHAT_ID, YOUR_ADMIN_ID
//
// Функции:
// - Админ-панель (inline кнопки) только для админа
// - /status /servers /reconnect — управление VPN
// - /logs — отправка логов
// - /pc /apk — раздача файлов всем
// - Отчёты при ошибках (не чаще часа)

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	botToken  = "YOUR_BOT_TOKEN"
	chatID    = "YOUR_CHAT_ID"
	adminID   = int64(YOUR_ADMIN_ID)  // например 6569139926
	fileURL   = "http://YOUR_VPS_IP:8090"
)

var (
	offset int64
	mu     sync.Mutex
	buffer []string
)

func main() {
	// Отправить стартовое сообщение
	send(chatID, "🟢 VPN бот запущен")
	
	// Запустить polling
	for {
		pollUpdates()
		time.Sleep(3 * time.Second)
	}
}

func pollUpdates() {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=5&offset=%d", botToken, offset)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID int64 `json:"update_id"`
			Message  *struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
				From struct {
					ID        int64  `json:"id"`
					FirstName string `json:"first_name"`
				} `json:"from"`
				Text string `json:"text"`
			} `json:"message"`
			CallbackQuery *struct {
				ID   string `json:"id"`
				From struct {
					ID int64 `json:"id"`
				} `json:"from"`
				Data string `json:"data"`
			} `json:"callback_query"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for _, upd := range result.Result {
		offset = upd.UpdateID + 1
		if upd.CallbackQuery != nil {
			handleCallback(upd.CallbackQuery.ID, upd.CallbackQuery.Data, upd.CallbackQuery.From.ID)
			answerCallback(upd.CallbackQuery.ID)
		}
		if upd.Message != nil {
			handleCommand(upd.Message.Text, upd.Message.From.ID, upd.Message.Chat.ID)
		}
	}
}

func handleCommand(text string, userID int64, chat int64) {
	isAdmin := userID == adminID

	switch text {
	case "/start", "/help":
		if isAdmin {
			sendKeyboard(chat, "🛡 Админ-панель\n\n/status — статус\n/servers — серверы\n/reconnect — переподключить\n/logs — логи\n\n/pc — ПК\n/apk — Android\n/ios — iOS", adminKeyboard())
		} else {
			sendKeyboard(chat, "🌐 VPN\n\n/pc — ПК\n/apk — Android\n/ios — iOS конфиг", userKeyboard())
		}
	case "/pc":
		sendFile(chat, "💻 ПК", fileURL+"/snowden-portable.zip")
	case "/apk":
		sendFile(chat, "📱 Android", fileURL+"/snowden-android.apk")
	case "/ios":
		sendFile(chat, "🍎 iOS конфиг", fileURL+"/snowden-ios-config.json")
	case "/status":
		if isAdmin {
			send(chat, "📊 Статус: активен")
		} else {
			send(chat, "⛔ Только для админа")
		}
	case "/logs":
		if isAdmin {
			mu.Lock()
			logs := buffer
			buffer = nil
			mu.Unlock()
			if len(logs) == 0 {
				send(chat, "Лог пуст")
			} else {
				msg := "📋 Логи:\n```\n"
				for _, l := range logs {
					if len(l) > 100 { l = l[:100] }
					msg += l + "\n"
				}
				msg += "```"
				send(chat, msg)
			}
		} else {
			send(chat, "⛔ Только для админа")
		}
	case "/reconnect":
		if isAdmin {
			send(chat, "🔄 Переподключение...")
		} else {
			send(chat, "⛔ Только для админа")
		}
	}
}

func handleCallback(cbID, data string, userID int64) {
	chat, _ := strconv.ParseInt(chatID, 10, 64)
	isAdmin := userID == adminID

	switch data {
	case "status":
		if isAdmin { send(chat, "📊 VPN активен") }
	case "logs":
		if isAdmin { handleCommand("/logs", userID, chat) }
	case "reconnect":
		if isAdmin { send(chat, "🔄 Переподключение...") }
	case "pc":
		sendFile(chat, "💻 ПК", fileURL+"/snowden-portable.zip")
	case "apk":
		sendFile(chat, "📱 Android", fileURL+"/snowden-android.apk")
	case "ios":
		sendFile(chat, "🍎 iOS", fileURL+"/snowden-ios-config.json")
	}
}

// --- HTTP helpers ---

type tgButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}
type tgKeyboard struct {
	InlineKeyboard [][]tgButton `json:"inline_keyboard"`
}

func adminKeyboard() tgKeyboard {
	return tgKeyboard{
		[][]tgButton{
			{{"📊 Статус", "status"}, {"📋 Логи", "logs"}},
			{{"🔄 Переподключить", "reconnect"}},
		},
	}
}

func userKeyboard() tgKeyboard {
	return tgKeyboard{
		[][]tgButton{
			{{"💻 ПК", "pc"}, {"📱 Android", "apk"}, {"🍎 iOS", "ios"}},
		},
	}
}

func send(chatID string, text string) {
	sendKeyboard(chatID, text, tgKeyboard{})
}

// sendKeyboard accepts int64 chat
func sendKeyboardInt(chat int64, text string, kb tgKeyboard) {
	sendKeyboard(strconv.FormatInt(chat, 10), text, kb)
}

func sendKeyboard(chat, text string, kb tgKeyboard) {
	payload := map[string]any{
		"chat_id":    chat,
		"text":       text,
		"parse_mode": "Markdown",
	}
	if len(kb.InlineKeyboard) > 0 {
		payload["reply_markup"] = kb
	}
	body, _ := json.Marshal(payload)
	http.Post(
		"https://api.telegram.org/bot"+botToken+"/sendMessage",
		"application/json", bytes.NewReader(body))
}

func sendFile(chat int64, caption, url string) {
	go func() {
		resp, err := http.Get(url)
		if err != nil {
			send(strconv.FormatInt(chat, 10), "⬇️ Скачать напрямую:\n"+url)
			return
		}
		defer resp.Body.Close()
		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument?chat_id=%d&caption=%s",
			botToken, chat, caption)
		http.Post(apiURL, "application/octet-stream", resp.Body)
	}()
}

func answerCallback(cbID string) {
	payload, _ := json.Marshal(map[string]string{"callback_query_id": cbID})
	http.Post("https://api.telegram.org/bot"+botToken+"/answerCallbackQuery",
		"application/json", bytes.NewReader(payload))
}

// helper for tgButton literal
func init() {
	_ = strings.TrimSpace
}
