import Foundation
import NetworkExtension
import Combine

// MARK: - VPN Manager
// Bridges Swift UI to GoCore (gomobile-generated framework).
// Full port of PC algorithms: Engine, Manager, AdaptiveEngine, CircuitBreaker, ErrorClassifier.
class VPNManager: ObservableObject {
    static let shared = VPNManager()
    
    @Published var status = VPNStatus()
    @Published var diagnostics = DiagStatus()
    @Published var logs: [String] = []
    
    private var goCore: SnowdenCore?
    private var tunnelProviderManager: NETunnelProviderManager?
    private var cancellables = Set<AnyCancellable>()
    private var statusTimer: Timer?
    private let logsQueue = DispatchQueue(label: "snowden.logs", qos: .utility)
    
    // MARK: - Initialization
    private init() {
        // Initialize GoCore (gomobile-generated)
        goCore = SnowdenCore()
        
        // Set callbacks from Go to Swift
        goCore?.setLogCallback { [weak self] line in
            self?.logsQueue.async {
                DispatchQueue.main.async {
                    self?.logs.append(line)
                    if (self?.logs.count ?? 0) > 500 {
                        self?.logs.removeFirst(self!.logs.count - 500)
                    }
                }
            }
        }
        
        goCore?.setDiagCallback { [weak self] line in
            self?.logsQueue.async {
                DispatchQueue.main.async {
                    self?.logs.append(line)
                }
            }
        }
    }
    
    // MARK: - Setup
    func setup() {
        loadTunnelProvider { [weak self] in
            self?.refreshStatus()
            self?.startStatusPolling()
        }
    }
    
    // MARK: - Toggle VPN
    func toggleVPN() {
        if status.connected {
            stopVPN()
        } else {
            startVPN()
        }
    }
    
    // MARK: - Start VPN
    func startVPN() {
        guard let goCore = goCore else {
            updateStatus(state: "error", message: "GoCore not initialized")
            return
        }
        
        // Build config from template (same as PC)
        let configJSON = buildConfig()
        
        // Start via GoCore (embedded sing-box)
        do {
            try goCore.startVPN("vps-reality", configJSON: configJSON)
            
            // Also start the Network Extension (iOS VPN tunnel)
            startTunnelProvider { [weak self] error in
                if let error = error {
                    self?.updateStatus(state: "error", message: "NE: \(error.localizedDescription)")
                }
            }
            
            // Start AdaptiveEngine health checks
            goCore.adaptiveStart("vps-reality", config: configJSON.data(using: .utf8) ?? Data())
            
            updateStatus(state: "starting", configId: "vps-reality")
            
        } catch {
            updateStatus(state: "error", message: error.localizedDescription)
        }
    }
    
    // MARK: - Stop VPN
    func stopVPN() {
        goCore?.adaptiveStop()
        
        do {
            try goCore?.stopVPN()
            stopTunnelProvider { [weak self] error in
                if let error = error {
                    self?.logs.append("[error] NE stop failed: \(error)")
                }
            }
            updateStatus(state: "stopped")
        } catch {
            updateStatus(state: "error", message: error.localizedDescription)
        }
    }
    
    // MARK: - Reload VPN
    func reloadVPN() {
        let configJSON = buildConfig()
        do {
            try goCore?.reloadVPN("vps-reality", configJSON: configJSON)
            logs.append("[info] Config reloaded")
        } catch {
            logs.append("[error] Reload failed: \(error)")
        }
    }
    
    // MARK: - Refresh Status
    func refreshStatus() {
        guard let goCore = goCore else { return }
        
        // Get status from GoCore
        if let statusJSON = goCore.status().data(using: .utf8),
           let st = try? JSONDecoder().decode(VPNStatus.self, from: statusJSON) {
            DispatchQueue.main.async {
                self.status = st
            }
        }
        
        // Get diagnostics from GoCore
        if let diagJSON = goCore.diagnostics().data(using: .utf8),
           let diag = try? JSONDecoder().decode(DiagStatus.self, from: diagJSON) {
            DispatchQueue.main.async {
                self.diagnostics = diag
            }
        }
    }
    
    // MARK: - Build Config (same as PC)
    private func buildConfig() -> String {
        guard let goCore = goCore else { return "" }
        
        // VPS config (same as PC template-vps-reality.json)
        let server = "192.109.206.234"
        let serverPort = 443
        let uuid = "1e0e52d1-7935-452c-a868-80308e7ab7d2"
        let serverName = "snowden-system.192-109-206-234.nip.io"
        let listenPort = 20808
        
        // Generate ru-cidr.json from bundled list
        let ruCIDRPath = ensureCIDRFile()
        
        do {
            let config = try goCore.buildConfigJSON(
                server, serverPort: serverPort, uuid: uuid,
                serverName: serverName, listenPort: listenPort, ruCIDRPath: ruCIDRPath
            )
            return config
        } catch {
            logs.append("[error] Build config failed: \(error)")
            return fallbackConfig()
        }
    }
    
    private func ensureCIDRFile() -> String {
        let docsDir = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first!
        let cidrPath = docsDir.appendingPathComponent("ru-cidr.json").path
        
        // If file exists, use it
        if FileManager.default.fileExists(atPath: cidrPath) {
            return cidrPath
        }
        
        // Otherwise create from bundled list (copy from app bundle)
        guard let bundlePath = Bundle.main.path(forResource: "ru-cidr", ofType: "lst") else {
            return ""
        }
        
        do {
            let rawList = try String(contentsOfFile: bundlePath, encoding: .utf8)
            let path = try goCore?.ensureCIDR(rawList, dir: docsDir.path)
            return path ?? ""
        } catch {
            logs.append("[error] CIDR file creation failed: \(error)")
            return ""
        }
    }
    
    private func fallbackConfig() -> String {
        // Minimal fallback if BuildConfig fails
        return """
        {
          "log": {"level": "info", "timestamp": true},
          "dns": {
            "servers": [
              {"type": "https", "tag": "cloudflare", "server": "1.1.1.1", "path": "/dns-query", "detour": "auto"},
              {"type": "local", "tag": "local", "detour": "direct"}
            ],
            "rules": [{"outbound": "any", "server": "local"}],
            "strategy": "ipv4_only"
          },
          "inbounds": [
            {"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 20808}
          ],
          "outbounds": [
            {"type": "urltest", "tag": "auto", "outbounds": ["proxy", "direct"], "url": "https://www.gstatic.com/generate_204", "interval": "1m", "tolerance": 100},
            {"type": "vless", "tag": "proxy", "server": "192.109.206.234", "server_port": 443, "uuid": "1e0e52d1-7935-452c-a868-80308e7ab7d2", "tls": {"enabled": true, "server_name": "snowden-system.192-109-206-234.nip.io"}},
            {"type": "direct", "tag": "direct"},
            {"type": "block", "tag": "block"}
          ],
          "route": {
            "rules": [
              {"action": "sniff"},
              {"action": "hijack-dns", "inbound": "mixed-in", "protocol": "dns"},
              {"ip_is_private": true, "action": "direct"}
            ],
            "final": "auto",
            "default_domain_resolver": "local",
            "auto_detect_interface": true
          }
        }
        """
    }
    
    // MARK: - Status Polling
    private func startStatusPolling() {
        statusTimer = Timer.scheduledTimer(withTimeInterval: 2.0, repeats: true) { [weak self] _ in
            self?.refreshStatus()
        }
    }
    
    // MARK: - Network Extension (iOS VPN Tunnel)
    private func loadTunnelProvider(completion: @escaping () -> Void) {
        NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, error in
            if let error = error {
                self?.logs.append("[error] NE load failed: \(error)")
                completion()
                return
            }
            
            if let manager = managers?.first {
                self?.tunnelProviderManager = manager
                completion()
            } else {
                // Create new manager
                let newManager = NETunnelProviderManager()
                let proto = NETunnelProviderProtocol()
                proto.providerBundleIdentifier = "com.snowdensystem.vpn"
                proto.serverAddress = "snowden.system"
                proto.providerConfiguration = ["config": self?.buildConfig() ?? ""]
                newManager.protocolConfiguration = proto
                newManager.localizedDescription = "Snowden System VPN"
                newManager.isEnabled = true
                
                newManager.saveToPreferences { error in
                    if let error = error {
                        self?.logs.append("[error] NE save failed: \(error)")
                    }
                    self?.tunnelProviderManager = newManager
                    completion()
                }
            }
        }
    }
    
    private func startTunnelProvider(completion: @escaping (Error?) -> Void) {
        guard let manager = tunnelProviderManager else {
            completion(NSError(domain: "SnowdenSystem", code: 1, userInfo: [NSLocalizedDescriptionKey: "NE manager not initialized"]))
            return
        }
        
        do {
            try manager.connection.startVPNTunnel()
            completion(nil)
        } catch {
            completion(error)
        }
    }
    
    private func stopTunnelProvider(completion: @escaping (Error?) -> Void) {
        tunnelProviderManager?.connection.stopVPNTunnel()
        completion(nil)
    }
    
    // MARK: - Helpers
    private func updateStatus(state: String, configId: String = "", message: String = "") {
        DispatchQueue.main.async {
            self.status = VPNStatus(
                state: state,
                configId: configId,
                message: message,
                connected: state == "running"
            )
        }
    }
}

// MARK: - SnowdenCore Bridge (gomobile-generated)
// This class is generated by gomobile bind. Declared here for compilation.
// In real build: import SnowdenCore
class SnowdenCore {
    func startVPN(_ configID: String, configJSON: String) throws {
        // Generated by gomobile
    }
    func stopVPN() throws {
        // Generated by gomobile
    }
    func reloadVPN(_ configID: String, configJSON: String) throws {
        // Generated by gomobile
    }
    func status() -> String {
        // Generated by gomobile
        return ""
    }
    func diagnostics() -> String {
        // Generated by gomobile
        return ""
    }
    func setLogCallback(_ cb: @escaping (String) -> Void) {
        // Generated by gomobile
    }
    func setDiagCallback(_ cb: @escaping (String) -> Void) {
        // Generated by gomobile
    }
    func buildConfigJSON(_ server: String, serverPort: Int, uuid: String, serverName: String, listenPort: Int, ruCIDRPath: String) throws -> String {
        // Generated by gomobile
        return ""
    }
    func ensureCIDR(_ rawList: String, dir: String) throws -> String {
        // Generated by gomobile
        return ""
    }
    func adaptiveStart(_ configID: String, config: Data) {
        // Generated by gomobile
    }
    func adaptiveStop() {
        // Generated by gomobile
    }
}
