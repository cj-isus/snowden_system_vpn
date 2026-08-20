import Foundation
import UIKit

/// TelegramReporter — отправка статуса VPN в Telegram-бот.
///
/// Аналог Android TelegramReporter.kt. Использует URLSession для HTTPS POST
/// к api.telegram.org (через VPN-туннель, т.к. Telegram API заблокирован в РФ).
///
/// События:
///   🟢 VPN подключён   — при старте туннеля
///   🔴 VPN отключён    — при остановке
///   ❌ Ошибка запуска  — при сбое + текст ошибки
class TelegramReporter {

    static let shared = TelegramReporter()

    // Bot credentials — те же что и на Android/ПК
    // Credentials are injected by a private build configuration, never source.
    private let botToken: String
    private let chatId: String

    private var deviceName: String
    private var lastReportTime: Date = .distantPast
    private let minReportGap: TimeInterval = 30 * 60 // 30 min

    private init() {
        botToken = Bundle.main.object(forInfoDictionaryKey: "SNOWDEN_TG_TOKEN") as? String ?? ""
        chatId = Bundle.main.object(forInfoDictionaryKey: "SNOWDEN_TG_CHAT_ID") as? String ?? ""
        deviceName = "\(UIDevice.current.name) (\(UIDevice.current.model))"
    }

    /// Отправить отчёт немедленно (start/stop/error).
    func report(_ event: String, details: String = "") {
        DispatchQueue.global(qos: .background).async { [weak self] in
            guard let self = self else { return }

            let timeFormatter = DateFormatter()
            timeFormatter.dateFormat = "HH:mm:ss"

            var msg = "📱 *snowden.system* — iOS\n"
            msg += "Устройство: `\(self.deviceName)`\n"
            msg += "⏱ \(timeFormatter.string(from: Date()))\n"
            msg += "\(event)\n"
            if !details.isEmpty {
                var d = details
                if d.count > 500 { d = String(d.prefix(500)) + "…" }
                msg += "```\n\(d)\n```"
            }

            self.send(msg)
        }
    }

    /// Троттленный OK-отчёт — не чаще раза в 30 мин.
    func reportOkIfDue() {
        let now = Date()
        if now.timeIntervalSince(lastReportTime) < minReportGap {
            return
        }
        lastReportTime = now
        report("✅ VPN работает стабильно")
    }

    /// HTTPS POST к api.telegram.org/bot.../sendMessage
    private func send(_ text: String) {
        guard !botToken.isEmpty, !chatId.isEmpty else {
            NSLog("[TelegramReporter] disabled: local credentials are not provisioned")
            return
        }
        let urlString = "https://api.telegram.org/bot\(botToken)/sendMessage"
        guard let url = URL(string: urlString) else { return }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json; charset=UTF-8", forHTTPHeaderField: "Content-Type")
        request.timeoutInterval = 15

        // JSON payload (ручное кодирование — без сторонних библиотек)
        let escaped = text
            .replacingOccurrences(of: "\\", with: "\\\\")
            .replacingOccurrences(of: "\"", with: "\\\"")
            .replacingOccurrences(of: "\n", with: "\\n")

        let payload = """
        {"chat_id":"\(chatId)","text":"\(escaped)","parse_mode":"Markdown"}
        """
        request.httpBody = payload.data(using: .utf8)

        let task = URLSession.shared.dataTask(with: request) { data, response, error in
            if let error = error {
                NSLog("[TelegramReporter] send failed: %@", error.localizedDescription)
                return
            }
            if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode != 200 {
                NSLog("[TelegramReporter] HTTP %d", httpResponse.statusCode)
            }
        }
        task.resume()
    }
}
