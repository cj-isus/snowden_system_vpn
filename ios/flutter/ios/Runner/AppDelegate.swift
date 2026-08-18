import Flutter
import NetworkExtension
import UIKit

/// AppDelegate — мост между Flutter (MethodChannel) и iOS Network Extension.
///
/// Контракт MethodChannel ИДЕНТИЧЕН Android:
///   "startVpn"  → {config: String} → настраивает NETunnelProviderManager
///   "stopVpn"   → останавливает туннель
///   "getStatus" → Bool (подключён/нет)
///   "onLog"     ← callback из расширения в Flutter
///
/// Благодаря идентичному контракту, lib/main.dart НЕ ТРЕБУЕТ ИЗМЕНЕНИЙ.
class AppDelegate: FlutterAppDelegate {

    private var vpnManager: NETunnelProviderManager?
    private var channel: FlutterMethodChannel?

    override func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        let controller = window?.rootViewController as? FlutterViewController
        channel = FlutterMethodChannel(
            name: "snowden.system/vpn",
            binaryMessenger: controller!.binaryMessenger
        )

        channel?.setMethodCallHandler { [weak self] call, result in
            switch call.method {
            case "startVpn":
                let config = call.arguments as? [String: Any]
                let configJson = config?["config"] as? String ?? ""
                self?.startVPN(config: configJson, result: result)

            case "stopVpn":
                self?.stopVPN(result: result)

            case "getStatus":
                let status = self?.vpnManager?.connection.status == .connected
                result(status)

            default:
                result(FlutterMethodNotImplemented)
            }
        }

        // Загружаем существующую VPN-конфигурацию (если уже настроена)
        loadExistingVPNConfig()

        return super.application(application, didFinishLaunchingWithOptions: launchOptions)
    }

    // MARK: - VPN Start

    private func startVPN(config: String, result: @escaping FlutterResult) {
        // Сначала загрузить все существующие конфигурации
        NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, error in
            guard let self = self else { return }

            if let error = error {
                NSLog("[snowden] loadAllFromPreferences error: %@", error.localizedDescription)
                result(false)
                return
            }

            // Найти или создать менеджер
            let manager = managers?.first ?? NETunnelProviderManager()
            self.vpnManager = manager

            // Настроить протокол
            let proto = NETunnelProviderProtocol()
            proto.providerBundleIdentifier = "com.snowden.system.PacketTunnel"
            proto.serverAddress = "snowden.system"
            proto.providerConfiguration = ["config": config]
            proto.disconnectOnSleep = false

            manager.protocolConfiguration = proto
            manager.localizedDescription = "snowden.system"
            manager.isEnabled = true

            manager.saveToPreferences { saveError in
                if let saveError = saveError {
                    NSLog("[snowden] saveToPreferences error: %@", saveError.localizedDescription)
                    result(false)
                    return
                }

                // Перезагружаем prefs (iOS требует)
                manager.loadFromPreferences { _ in
                    do {
                        // Остановить если уже запущен
                        if manager.connection.status == .connected ||
                           manager.connection.status == .connecting {
                            try manager.connection.stopVPNTunnel()
                            Thread.sleep(forTimeInterval: 0.5)
                        }

                        // Запустить туннель
                        let session = manager.connection as? NETunnelProviderSession
                        try session?.startTunnel(options: nil)
                        NSLog("[snowden] tunnel started")
                        result(true)
                    } catch {
                        NSLog("[snowden] startTunnel error: %@", error.localizedDescription)
                        result(false)
                    }
                }
            }
        }
    }

    // MARK: - VPN Stop

    private func stopVPN(result: @escaping FlutterResult) {
        guard let manager = vpnManager else {
            result(false)
            return
        }

        do {
            try (manager.connection as? NETunnelProviderSession)?.stopVPNTunnel()
            NSLog("[snowden] tunnel stopped")
            result(true)
        } catch {
            NSLog("[snowden] stopTunnel error: %@", error.localizedDescription)
            result(false)
        }
    }

    // MARK: - Load existing

    private func loadExistingVPNConfig() {
        NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, _ in
            self?.vpnManager = managers?.first
        }
    }
}
