import NetworkExtension
import os.log

// MARK: - Packet Tunnel Provider
// iOS Network Extension that creates the VPN tunnel.
// Communicates with GoCore (embedded sing-box) for actual proxy logic.
class SnowdenPacketTunnelProvider: NEPacketTunnelProvider {
    
    private let logger = OSLog(subsystem: "com.snowdensystem.vpn", category: "Tunnel")
    private var goCore: SnowdenCore?
    private var isConnected = false
    
    override func startTunnel(options: [String : NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        os_log("Starting Snowden tunnel", log: logger, type: .info)
        
        // Get config from provider configuration
        guard let configJSON = (options?["config"] as? String) ?? (protocolConfiguration as? NETunnelProviderProtocol)?.providerConfiguration?["config"] as? String else {
            os_log("No config provided", log: logger, type: .error)
            completionHandler(NSError(domain: "SnowdenSystem", code: 1, userInfo: [NSLocalizedDescriptionKey: "No config provided"]))
            return
        }
        
        // Initialize GoCore
        goCore = SnowdenCore()
        
        // Set log callback
        goCore?.setLogCallback { [weak self] line in
            os_log("%{public}@", log: self?.logger ?? .default, type: .info, line)
        }
        
        // Start GoCore (embedded sing-box)
        do {
            try goCore?.startVPN("vps-reality", configJSON: configJSON)
            isConnected = true
            
            // Configure tunnel settings
            let tunnelSettings = configureTunnelSettings()
            setTunnelNetworkSettings(tunnelSettings) { error in
                if let error = error {
                    os_log("Tunnel settings failed: %{public}@", log: self.logger, type: .error, error.localizedDescription)
                    self.stopTunnel(with: .none, completionHandler: {})
                    completionHandler(error)
                } else {
                    os_log("Tunnel started successfully", log: self.logger, type: .info)
                    completionHandler(nil)
                }
            }
            
        } catch {
            os_log("GoCore start failed: %{public}@", log: logger, type: .error, error.localizedDescription)
            completionHandler(error)
        }
    }
    
    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        os_log("Stopping Snowden tunnel, reason: %d", log: logger, type: .info, reason.rawValue)
        
        isConnected = false
        
        // Stop GoCore
        do {
            try goCore?.stopVPN()
        } catch {
            os_log("GoCore stop error: %{public}@", log: logger, type: .error, error.localizedDescription)
        }
        
        goCore = nil
        
        // Clear tunnel settings
        setTunnelNetworkSettings(nil) { _ in
            completionHandler()
        }
    }
    
    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)?) {
        // Handle messages from main app
        guard let message = String(data: messageData, encoding: .utf8) else {
            completionHandler?(nil)
            return
        }
        
        os_log("App message: %{public}@", log: logger, type: .info, message)
        
        switch message {
        case "status":
            let status = goCore?.status() ?? "{}"
            completionHandler?(status.data(using: .utf8))
            
        case "diagnostics":
            let diag = goCore?.diagnostics() ?? "{}"
            completionHandler?(diag.data(using: .utf8))
            
        case "reload":
            // Reload config
            if let configJSON = (protocolConfiguration as? NETunnelProviderProtocol)?.providerConfiguration?["config"] as? String {
                do {
                    try goCore?.reloadVPN("vps-reality", configJSON: configJSON)
                    completionHandler?("{\"ok\": true}".data(using: .utf8))
                } catch {
                    completionHandler?("{\"ok\": false, \"error\": \"\(error)\"}".data(using: .utf8))
                }
            }
            
        default:
            completionHandler?(nil)
        }
    }
    
    // MARK: - Tunnel Settings
    private func configureTunnelSettings() -> NEPacketTunnelNetworkSettings {
        let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "snowden.system")
        
        // IPv4 settings
        let ipv4Settings = NEIPv4Settings(addresses: ["198.18.0.1"], subnetMasks: ["255.255.0.0"])
        ipv4Settings.includedRoutes = [NEIPv4Route.default()]
        settings.ipv4Settings = ipv4Settings
        
        // DNS settings (sing-box handles DNS hijacking)
        let dnsSettings = NEDNSSettings(servers: ["127.0.0.1"])
        dnsSettings.matchDomains = [""]
        settings.dnsSettings = dnsSettings
        
        // MTU
        settings.mtu = NSNumber(value: 1280)
        
        // Proxy settings (route through sing-box mixed-in)
        let proxySettings = NEProxySettings()
        proxySettings.httpEnabled = true
        proxySettings.httpServer = NEProxyServer(address: "127.0.0.1", port: 20808)
        proxySettings.httpsEnabled = true
        proxySettings.httpsServer = NEProxyServer(address: "127.0.0.1", port: 20808)
        proxySettings.autoProxyConfigurationEnabled = false
        proxySettings.excludeSimpleHostnames = true
        settings.proxySettings = proxySettings
        
        return settings
    }
}
