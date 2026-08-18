import NetworkExtension
import Libbox
import os.log
import Foundation

/// NEPacketTunnelProvider — iOS-аналог Android VpnService.
///
/// Живёт в отдельном процессе (Network Extension). Управляется через
/// NETunnelProviderManager из основного приложения.
///
/// Ключевые отличия от Android:
/// 1. Нет protect() — iOS автоматически биндит outbound к physical interface
/// 2. Нет addDisallowedApplication — банковские исключения в route rules sing-box
/// 3. TUN работает через NEPacketTunnelNetworkSettings + packetFlow
/// 4. setFixAndroidStack НЕ вызывается (iOS-only)
class PacketTunnelProvider: NEPacketTunnelProvider {

    private let log = OSLog(subsystem: "com.snowden.system", category: "PacketTunnel")
    private var boxService: BoxService?
    private var commandClient: LibboxCommandClient?
    private var platformInterface: BoxPlatformInterface?

    // MARK: - Lifecycle

    /// Вызывается когда пользователь нажимает "Подключить" (через NETunnelProviderSession.startTunnel).
    /// providerProtocol.providerConfiguration содержит JSON-конфиг sing-box.
    override func startTunnel(options: [String: NSObject]?,
                              completionHandler: @escaping (Error?) -> Void) {
        os_log(">>> startTunnel", log: log, type: .info)

        // Читаем конфиг из providerConfiguration (передаётся из AppDelegate)
        guard let protocolConfig = self.protocolConfiguration as? NETunnelProviderProtocol,
              let providerConfig = protocolConfig.providerConfiguration,
              let configJson = providerConfig["config"] as? String else {
            os_log("!!! no config in providerConfiguration", log: log, type: .error)
            completionHandler(VpnError.noConfig)
            return
        }

        // Сохранить конфиг в App Group для отладки
        saveConfigToSharedContainer(configJson)

        // 1. Настроить libbox
        do {
            try setupLibbox()
        } catch {
            os_log("!!! libbox setup failed: %{public}@", log: log, type: .error, error.localizedDescription)
            completionHandler(error)
            return
        }

        // 2. Создать platform interface (обёртка над packetFlow)
        platformInterface = BoxPlatformInterface(provider: self)
        guard let platform = platformInterface else {
            completionHandler(VpnError.interfaceError)
            return
        }

        // 3. Настроить туннель (IP, маршруты, DNS)
        let networkSettings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "127.0.0.1")
        let ipv4Settings = NEIPv4Settings(addresses: ["172.19.0.1"], subnetMasks: ["255.255.255.252"])
        ipv4Settings.includedRoutes = [NEIPv4Route.default()]
        networkSettings.ipv4Settings = ipv4Settings
        networkSettings.dnsSettings = NEDNSSettings(servers: ["8.8.8.8", "1.1.1.1"])
        networkSettings.mtu = 1500

        setTunnelNetworkSettings(networkSettings) { [weak self] error in
            guard let self = self else { return }

            if let error = error {
                os_log("!!! setTunnelNetworkSettings failed: %{public}@", log: self.log, type: .error, error.localizedDescription)
                completionHandler(error)
                return
            }

            os_log(">>> tunnel network settings configured", log: self.log, type: .info)

            // 4. Запустить sing-box ядро
            do {
                self.boxService = try Libbox.newBoxService(configJson, platform)
                try self.boxService?.start()

                // 5. Запустить чтение пакетов из packetFlow → sing-box
                self.startPacketReading()

                // 6. Логирование ядра
                self.startLogClient()

                // 7. Уведомить Telegram
                TelegramReporter.shared.report("🟢 VPN подключён")

                completionHandler(nil)
            } catch {
                os_log("!!! boxService.start failed: %{public}@", log: self.log, type: .error, error.localizedDescription)
                TelegramReporter.shared.report("❌ Ошибка запуска VPN", details: error.localizedDescription)
                completionHandler(error)
            }
        }
    }

    /// Вызывается при отключении VPN пользователем.
    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        os_log(">>> stopTunnel reason: %d", log: log, type: .info, reason.rawValue)

        do {
            try boxService?.close()
        } catch {
            os_log("!!! boxService.close error: %{public}@", log: log, type: .error, error.localizedDescription)
        }

        boxService = nil
        commandClient = nil
        platformInterface = nil

        TelegramReporter.shared.report("🔴 VPN отключён")
        completionHandler()
    }

    // MARK: - libbox setup

    private func setupLibbox() throws {
        let setupOptions = LibboxSetupOptions()

        // App Group container — shared между app и Network Extension
        let sharedContainer = FileManager.default.containerURL(
            forSecurityApplicationGroupIdentifier: "group.com.snowden.system"
        )
        let basePath = sharedContainer?.path ?? NSTemporaryDirectory()

        setupOptions.basePath = basePath
        setupOptions.workingPath = basePath
        setupOptions.tempPath = NSTemporaryDirectory()

        // ВНИМАНИЕ: НЕ вызываем setFixAndroidStack — это Android-only флаг.
        // iOS работает с TUN через utun, создаваемый Network Extension.

        try Libbox.setup(setupOptions)
        os_log(">>> libbox setup done (basePath=%{public}@)", log: log, type: .info, basePath)
    }

    // MARK: - Packet flow (чтение/запись пакетов)

    /// Цикл чтения пакетов из packetFlow и передачи в sing-box.
    /// sing-box обрабатывает их и возвращает ответные через writePackets.
    private func startPacketReading() {
        os_log(">>> startPacketReading", log: log, type: .info)

        // NEPacketTunnelProvider.packetFlow.readPackets() → sing-box inbound
        // libbox BoxPlatformInterface.openTun возвращает fd, через который
        // sing-box читает/пишет пакеты. На iOS мы используем packetFlow напрямую.
        packetFlow.readPackets { [weak self] packets, protocols in
            guard let self = self, let iface = self.platformInterface else { return }

            // Передаём пакеты в sing-box через platform interface
            for (i, packet) in packets.enumerated() {
                let proto = protocols[i]
                iface.writePacket(packet, protocolFamily: proto)
            }

            // Продолжаем читать
            self.startPacketReading()
        }
    }

    // MARK: - Log client

    /// CommandClient(CommandLog) — стриминг логов из ядра sing-box.
    private func startLogClient() {
        let options = LibboxCommandClientOptions()
        options.addCommand(LibboxCommandLog)
        options.statusInterval = 0

        commandClient = LibboxCommandClient(CommandLogHandler(provider: self), options)
        do {
            try commandClient?.connect()
            os_log(">>> log client connected", log: log, type: .info)
        } catch {
            os_log("!!! log client failed: %{public}@", log: log, type: .error, error.localizedDescription)
        }
    }

    // MARK: - Shared container

    private func saveConfigToSharedContainer(_ config: String) {
        guard let container = FileManager.default.containerURL(
            forSecurityApplicationGroupIdentifier: "group.com.snowden.system"
        ) else { return }

        let configFile = container.appendingPathComponent("config.json")
        try? config.write(to: configFile, atomically: true, encoding: .utf8)
        os_log(">>> config saved to %{public}@", log: log, type: .info, configFile.path)
    }

    /// Запись лога в shared container (для отладки) и отправка в app.
    func sendLog(_ message: String) {
        os_log("%{public}@", log: log, type: .info, message)

        // Запись в файл (App Group)
        guard let container = FileManager.default.containerURL(
            forSecurityApplicationGroupIdentifier: "group.com.snowden.system"
        ) else { return }

        let logFile = container.appendingPathComponent("vpn.log")
        let timestamp = ISO8601DateFormatter().string(from: Date())
        let line = "[\(timestamp)] \(message)\n"

        if let existing = try? String(contentsOf: logFile, encoding: .utf8) {
            try? (existing + line).write(to: logFile, atomically: true, encoding: .utf8)
        } else {
            try? line.write(to: logFile, atomically: true, encoding: .utf8)
        }
    }
}

// MARK: - CommandLogHandler

/// Обработчик логов sing-box для CommandClient.
class CommandLogHandler: NSObject, LibboxCommandClientHandler {
    private weak let provider: PacketTunnelProvider?

    init(provider: PacketTunnelProvider) {
        self.provider = provider
    }

    func connected() {
        provider?.sendLog("[core] log client connected")
    }

    func disconnected(_ message: String?) {
        provider?.sendLog("[core] log client disconnected: \(message ?? "")")
    }

    func writeLogs(_ logs: LibboxLogIterator?) {
        guard let logs = logs else { return }
        while logs.hasNext() {
            if let entry = logs.next() {
                let level = entry.level
                let levelStr: String
                switch level {
                case 0: levelStr = "PANIC"
                case 1: levelStr = "FATAL"
                case 2: levelStr = "ERROR"
                case 3: levelStr = "WARN"
                case 4: levelStr = "INFO"
                case 5: levelStr = "DEBUG"
                case 6: levelStr = "TRACE"
                default: levelStr = "L\(level)"
                }
                provider?.sendLog("[core:\(levelStr)] \(entry.message)")
            }
        }
    }

    func clearLogs() {}
    func initializeClashMode(_ p0: LibboxStringIterator?, _ p1: String?) {}
    func setDefaultLogLevel(_ p0: Int32) {}
    func updateClashMode(_ p0: String?) {}
    func writeConnectionEvents(_ p0: LibboxConnectionEvents?) {}
    func writeDNSQuery(_ p0: LibboxDnsQuery?) {}
    func writeGroups(_ p0: LibboxOutboundGroupIterator?) {}
    func writeOutbounds(_ p0: LibboxOutboundGroupItemIterator?) {}
    func writeStatus(_ p0: LibboxStatusMessage?) {}
}

// MARK: - Errors

enum VpnError: Error, LocalizedError {
    case noConfig
    case interfaceError

    var errorDescription: String? {
        switch self {
        case .noConfig: return "No VPN configuration found"
        case .interfaceError: return "Failed to create platform interface"
        }
    }
}
