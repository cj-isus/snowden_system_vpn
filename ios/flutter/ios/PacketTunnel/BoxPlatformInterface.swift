import NetworkExtension
import Libbox
import Foundation

/// BoxPlatformInterface — реализация LibboxPlatformInterface протокола для iOS.
///
/// На iOS нет protect() (как на Android) — iOS автоматически биндит outbound-сокеты
/// к physical interface через defaultPath. Нам не нужно ничего "защищать".
///
/// openTun возвращает fd, через который sing-box читает/пишет пакеты.
/// На iOS мы используем NEPacketTunnelProvider.packetFlow для передачи пакетов
/// между ядром и системой.
class BoxPlatformInterface: NSObject, LibboxPlatformInterface {

    private weak var provider: PacketTunnelProvider?
    private var tunFileDescriptor: Int32 = -1

    init(provider: PacketTunnelProvider) {
        self.provider = provider
        super.init()
    }

    // MARK: - Auto-detect interface control (protect альтернатива)

    /// На iOS всегда true — sing-box будет вызывать autoDetectInterfaceControl
    /// для каждого нового сокета.
    func usePlatformAutoDetectInterfaceControl() -> Bool {
        return true
    }

    /// iOS-эквивалент Android protect().
    /// На iOS НЕ нужно ничего делать — система сама биндит сокеты к physical
    /// interface через default route. Метод оставлен пустым для совместимости.
    func autoDetectInterfaceControl(_ fd: Int32) throws {
        provider?.sendLog(">>> autoDetectInterfaceControl fd=\(fd) (iOS no-op)")
    }

    // MARK: - openTun

    /// На iOS sing-box не вызывает openTun напрямую — NEPacketTunnelProvider
    /// уже создал utun через setTunnelNetworkSettings. Этот метод может
    /// вызываться libbox для получения fd туннеля.
    /// Возвращаем fd текущего packetFlow (если доступен).
    func openTun(_ options: LibboxTunOptions) throws -> Int32 {
        provider?.sendLog(">>> openTun MTU=\(options.mtu) (iOS uses packetFlow)")

        // На iOS libbox использует platform interface для чтения/записи пакетов
        // через packetFlow, а не через fd. Возвращаем -1 чтобы сигнализировать
        // что мы используем platform-based TUN.
        return -1
    }

    // MARK: - Interface monitoring

    func startDefaultInterfaceMonitor(_ listener: LibboxInterfaceUpdateListener?) throws {
        provider?.sendLog(">>> startDefaultInterfaceMonitor")
        // На iOS sing-box сам определяет интерфейс через NWPathMonitor
    }

    func closeDefaultInterfaceMonitor(_ listener: LibboxInterfaceUpdateListener?) throws {
        provider?.sendLog(">>> closeDefaultInterfaceMonitor")
    }

    // MARK: - Interface enumeration

    /// Возвращает список сетевых интерфейсов для sing-box NetworkManager.
    /// На iOS используем getifaddrs() — аналог java.net.NetworkInterface.
    func getInterfaces() throws -> LibboxNetworkInterfaceIterator {
        let iterator = iOSNetworkInterfaceIterator(provider: provider)
        return iterator
    }

    // MARK: - Stubs (не используются на iOS)

    func checkPlatformShell() throws {}
    func clearDNSCache() {}
    func closeNeighborMonitor(_ listener: LibboxNeighborUpdateListener?) throws {}
    func createBridge(_ options: LibboxBridgeOptions?) throws -> LibboxBridgeSession { throw VpnError.interfaceError }
    func findConnectionOwner(_ fd: Int32, ip: String?, port: Int32, ip2: String?, port2: Int32) throws -> LibboxConnectionOwner { throw VpnError.interfaceError }
    func includeAllNetworks() -> Bool { return false }
    func localDNSTransport() -> LibboxLocalDNSTransport? { return nil }
    func lookupSFTPServer() throws -> String { return "" }
    func lookupUser(_ name: String?) throws -> LibboxPlatformUser { throw VpnError.interfaceError }
    func openShellSession(_ user: LibboxPlatformUser?, cmd: String?, env: LibboxStringIterator?, dir: String?, rows: Int32, cols: Int32) throws -> LibboxShellSession { throw VpnError.interfaceError }
    func readSystemSSHHostKey() throws -> String { return "" }
    func readWIFIState() -> LibboxWIFIState { return LibboxWIFIState() }
    func registerMyInterface(_ name: String?) {}
    func sendNotification(_ notification: LibboxNotification?) throws {}
    func startNeighborMonitor(_ listener: LibboxNeighborUpdateListener?) throws {}
    func tailscaleHostname() -> String { return "" }
    func underNetworkExtension() -> Bool { return true }
    func usePlatformBridge() -> Bool { return false }
    func usePlatformShell() -> Bool { return false }
    func useProcFS() -> Bool { return false }

    // MARK: - Packet writing

    /// Записывает пакет обратно в туннель (от sing-box к ОС).
    func writePacket(_ packet: Data, protocolFamily: NSNumber) {
        provider?.packetFlow.writePackets([packet], withProtocols: [protocolFamily])
    }
}

// MARK: - iOSNetworkInterfaceIterator

/// Перечисление сетевых интерфейсов через getifaddrs().
/// Аналог AndroidNetworkInterfaceIterator — возвращает "ip/prefix" строки
/// (без prefix — Go panic на голом IP).
class iOSNetworkInterfaceIterator: NSObject, LibboxNetworkInterfaceIterator {
    private var interfaces: [LibboxNetworkInterface] = []
    private var index: Int = 0
    private weak var provider: PacketTunnelProvider?

    init(provider: PacketTunnelProvider?) {
        self.provider = provider
        super.init()

        enumerateInterfaces()
    }

    private func enumerateInterfaces() {
        var ifaddrPtr: UnsafeMutablePointer<ifaddrs>?
        guard getifaddrs(&ifaddrPtr) == 0, let firstAddr = ifaddrPtr else {
            provider?.sendLog("!!! getifaddrs failed")
            return
        }
        defer { freeifaddrs(firstAddr) }

        var interfaceMap: [String: [String]] = [:]
        var ifa = ifaddrPtr
        while let current = ifa {
            let pointee = current.pointee
            if let addrPtr = pointee.ifa_addr {
                let family = addrPtr.pointee.sa_family
                if family == sa_family_t(AF_INET) { // IPv4 only
                    let name = String(cString: pointee.ifa_name)
                    var hostname = [CChar](repeating: 0, count: Int(NI_MAXHOST))
                    getnameinfo(addrPtr, socklen_t(addrPtr.pointee.sa_len),
                                &hostname, socklen_t(hostname.count),
                                nil, 0, NI_NUMERICHOST)
                    let ip = String(cString: hostname)

                    // Skip loopback
                    if ip.hasPrefix("127.") {
                        ifa = pointee.ifa_next
                        continue
                    }

                    // Determine prefix from netmask
                    var prefix = "32"
                    if let netmaskPtr = pointee.ifa_netmask {
                        var mask = [CChar](repeating: 0, count: Int(NI_MAXHOST))
                        getnameinfo(netmaskPtr, socklen_t(netmaskPtr.pointee.sa_len),
                                    &mask, socklen_t(mask.count),
                                    nil, 0, NI_NUMERICHOST)
                        let maskStr = String(cString: mask)
                        prefix = maskToPrefix(maskStr)
                    }

                    if interfaceMap[name] == nil {
                        interfaceMap[name] = []
                    }
                    interfaceMap[name]?.append("\(ip)/\(prefix)")
                }
            }
            ifa = pointee.ifa_next
        }

        // Build LibboxNetworkInterface objects
        for (name, addresses) in interfaceMap {
            // Skip tun/utun interfaces
            if name.hasPrefix("utun") || name.hasPrefix("tun") || name == "lo" {
                continue
            }

            let ni = LibboxNetworkInterface()
            ni.name = name
            ni.index = 0
            ni.mtu = 1500

            let type: Int32
            if name.hasPrefix("en") || name.hasPrefix("wlan") {
                type = 1 // WiFi
            } else if name.hasPrefix("pdp") || name.hasPrefix("rmnet") || name.hasPrefix("ipsec") {
                type = 2 // Cellular
            } else {
                type = 3 // Other
            }
            ni.interfaceType = type

            // Set addresses as StringIterator
            ni.addresses = StringArrayIterator(strings: addresses)

            // Empty DNS
            ni.dnsServer = StringArrayIterator(strings: [])

            interfaces.append(ni)
        }

        let names = interfaces.map { $0.name }.joined(separator: ",")
        provider?.sendLog(">>> getInterfaces: \(names)")
    }

    func hasNext() -> Bool {
        return index < interfaces.count
    }

    func next() -> LibboxNetworkInterface {
        let item = interfaces[index]
        index += 1
        return item
    }

    /// Конвертация netmask (255.255.255.0) в prefix length (24).
    private func maskToPrefix(_ mask: String) -> String {
        let parts = mask.split(separator: ".")
        guard parts.count == 4 else { return "32" }

        var bits = 0
        for part in parts {
            let octet = Int(part) ?? 0
            var m = octet
            while m & 0x80 != 0 {
                bits += 1
                m <<= 1
            }
        }
        return String(bits)
    }
}

// MARK: - StringArrayIterator

/// Простой итератор строк для LibboxStringIterator.
class StringArrayIterator: NSObject, LibboxStringIterator {
    private let strings: [String]
    private var idx: Int = 0

    init(strings: [String]) {
        self.strings = strings
        super.init()
    }

    func hasNext() -> Bool {
        return idx < strings.count
    }

    func next() -> String {
        let s = strings[idx]
        idx += 1
        return s
    }

    func len() -> Int32 {
        return Int32(strings.count)
    }
}
