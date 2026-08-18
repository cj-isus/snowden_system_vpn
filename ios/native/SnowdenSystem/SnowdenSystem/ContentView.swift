import SwiftUI
import NetworkExtension

// MARK: - Snowden System iOS App
// Full port of PC algorithms: VLESS+Hysteria2+urltest, split-tunneling, adaptive engine
// UI: single logo button. Under the hood: everything smart and complex.

@main
struct SnowdenSystemApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    
    var body: some Scene {
        WindowGroup {
            ContentView()
                .preferredColorScheme(.dark)
        }
    }
}

class AppDelegate: NSObject, UIApplicationDelegate {
    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        // Initialize VPN manager on app launch
        VPNManager.shared.setup()
        return true
    }
    
    func applicationDidBecomeActive(_ application: UIApplication) {
        VPNManager.shared.refreshStatus()
    }
}

// MARK: - Content View (Single Logo Button)
struct ContentView: View {
    @StateObject private var vpnManager = VPNManager.shared
    @State private var showLogs = false
    @State private var showDiagnostics = false
    
    var body: some View {
        ZStack {
            // Background: terminal black
            Color.black.ignoresSafeArea()
            
            // Animated matrix-like particles (subtle)
            MatrixParticlesView()
                .opacity(0.15)
            
            VStack(spacing: 0) {
                // Top status bar
                StatusBarView(status: vpnManager.status)
                    .padding(.top, 8)
                
                Spacer()
                
                // Main logo button
                LogoButton(
                    isConnected: vpnManager.status.connected,
                    isConnecting: vpnManager.status.state == "starting" || vpnManager.status.state == "stopping",
                    stateColor: stateColor,
                    onTap: {
                        vpnManager.toggleVPN()
                    }
                )
                .padding(.bottom, 40)
                
                // Status text
                Text(statusText)
                    .font(.system(size: 14, weight: .semibold, design: .monospaced))
                    .foregroundColor(stateColor)
                    .padding(.bottom, 8)
                
                // Config name
                if !vpnManager.status.configId.isEmpty {
                    Text(vpnManager.status.configId)
                        .font(.system(size: 11, weight: .medium, design: .monospaced))
                        .foregroundColor(Color(hex: "#6b7280"))
                        .padding(.bottom, 4)
                }
                
                // Last error
                if !vpnManager.status.message.isEmpty {
                    Text(vpnManager.status.message)
                        .font(.system(size: 11, weight: .regular))
                        .foregroundColor(Color(hex: "#f87171"))
                        .multilineTextAlignment(.center)
                        .padding(.horizontal, 32)
                        .padding(.bottom, 16)
                }
                
                Spacer()
                
                // Bottom toolbar
                HStack(spacing: 24) {
                    Button(action: { showDiagnostics = true }) {
                        Image(systemName: "stethoscope")
                            .font(.system(size: 20))
                            .foregroundColor(Color(hex: "#34d399"))
                    }
                    
                    Button(action: { showLogs = true }) {
                        Image(systemName: "terminal")
                            .font(.system(size: 20))
                            .foregroundColor(Color(hex: "#34d399"))
                    }
                    
                    Button(action: { VPNManager.shared.refreshStatus() }) {
                        Image(systemName: "arrow.clockwise")
                            .font(.system(size: 20))
                            .foregroundColor(Color(hex: "#34d399"))
                    }
                }
                .padding(.bottom, 24)
            }
        }
        .sheet(isPresented: $showLogs) {
            LogsView(logs: vpnManager.logs)
        }
        .sheet(isPresented: $showDiagnostics) {
            DiagnosticsView(diagnostics: vpnManager.diagnostics)
        }
    }
    
    private var stateColor: Color {
        switch vpnManager.status.state {
        case "running": return Color(hex: "#34d399")
        case "starting", "stopping": return Color(hex: "#fbbf24")
        case "error": return Color(hex: "#f87171")
        default: return Color(hex: "#6b7280")
        }
    }
    
    private var statusText: String {
        switch vpnManager.status.state {
        case "running": return "Подключено"
        case "starting": return "Подключение…"
        case "stopping": return "Отключение…"
        case "error": return "Ошибка"
        default: return "Отключено"
        }
    }
}

// MARK: - Logo Button (The UI)
struct LogoButton: View {
    let isConnected: Bool
    let isConnecting: Bool
    let stateColor: Color
    let onTap: () -> Void
    
    @State private var isPressed = false
    @State private var pulseScale: CGFloat = 1.0
    
    var body: some View {
        Button(action: onTap) {
            ZStack {
                // Outer glow ring
                Circle()
                    .stroke(stateColor.opacity(0.3), lineWidth: 2)
                    .frame(width: 200, height: 200)
                    .scaleEffect(pulseScale)
                    .opacity(isConnected ? 1 : 0)
                
                // Main button circle
                Circle()
                    .fill(
                        LinearGradient(
                            colors: [
                                isConnected ? stateColor.opacity(0.15) : Color(hex: "#1a1a2e"),
                                Color(hex: "#0a0a0f")
                            ],
                            startPoint: .top,
                            endPoint: .bottom
                        )
                    )
                    .frame(width: 160, height: 160)
                    .overlay(
                        Circle()
                            .stroke(stateColor, lineWidth: isConnected ? 2 : 1)
                    )
                    .shadow(
                        color: stateColor.opacity(isConnected ? 0.4 : 0),
                        radius: isConnected ? 20 : 0,
                        x: 0, y: 0
                    )
                
                // Logo image (Pepe/Snowden)
                Image("snowden_logo")
                    .resizable()
                    .scaledToFit()
                    .frame(width: 100, height: 100)
                    .opacity(isConnecting ? 0.7 : 1.0)
                    
                // Connecting spinner
                if isConnecting {
                    ProgressView()
                        .progressViewStyle(CircularProgressViewStyle(tint: stateColor))
                        .scaleEffect(1.5)
                }
                
                // Status dot
                Circle()
                    .fill(stateColor)
                    .frame(width: 12, height: 12)
                    .offset(x: 50, y: 50)
                    .overlay(
                        Circle()
                            .stroke(Color.black, lineWidth: 2)
                            .frame(width: 12, height: 12)
                            .offset(x: 50, y: 50)
                    )
            }
            .scaleEffect(isPressed ? 0.92 : 1.0)
            .animation(.easeOut(duration: 0.15), value: isPressed)
            .animation(.easeInOut(duration: 1.5).repeatForever(autoreverses: true), value: pulseScale)
        }
        .buttonStyle(PlainButtonStyle())
        .disabled(isConnecting)
        .onAppear {
            if isConnected {
                withAnimation(.easeInOut(duration: 1.5).repeatForever(autoreverses: true)) {
                    pulseScale = 1.15
                }
            }
        }
        .onChange(of: isConnected) { connected in
            if connected {
                withAnimation(.easeInOut(duration: 1.5).repeatForever(autoreverses: true)) {
                    pulseScale = 1.15
                }
            } else {
                pulseScale = 1.0
            }
        }
        .pressEvents {
            withAnimation(.easeOut(duration: 0.1)) {
                isPressed = true
            }
        } onRelease: {
            withAnimation(.easeOut(duration: 0.1)) {
                isPressed = false
            }
        }
    }
}

// MARK: - Status Bar
struct StatusBarView: View {
    let status: VPNStatus
    
    var body: some View {
        HStack {
            HStack(spacing: 6) {
                Circle()
                    .fill(status.connected ? Color(hex: "#34d399") : Color(hex: "#6b7280"))
                    .frame(width: 8, height: 8)
                Text(status.connected ? "СОЕДИНЕНИЕ АКТИВНО" : "ОТКЛЮЧЕНО")
                    .font(.system(size: 10, weight: .bold, design: .monospaced))
                    .foregroundColor(status.connected ? Color(hex: "#34d399") : Color(hex: "#6b7280"))
            }
            
            Spacer()
            
            Text("snowden.system")
                .font(.system(size: 10, weight: .medium, design: .monospaced))
                .foregroundColor(Color(hex: "#6b7280"))
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(Color(hex: "#0d0d12"))
        .overlay(
            Rectangle()
                .frame(height: 1)
                .foregroundColor(Color(hex: "#2a2a35")),
            alignment: .bottom
        )
    }
}

// MARK: - Matrix Particles Background
struct MatrixParticlesView: View {
    let particles = (0..<30).map { _ in MatrixParticle() }
    
    var body: some View {
        TimelineView(.animation(minimumInterval: 0.05, paused: false)) { timeline in
            Canvas { context, size in
                for particle in particles {
                    var path = Path()
                    let x = particle.x * size.width
                    let y = (particle.y + timeline.date.timeIntervalSinceReferenceDate * particle.speed).truncatingRemainder(dividingBy: 1.0) * size.height
                    path.addRect(CGRect(x: x, y: y, width: 2, height: particle.length))
                    context.fill(path, with: .color(Color(hex: "#34d399").opacity(particle.opacity)))
                }
            }
        }
    }
}

struct MatrixParticle {
    let x: Double = Double.random(in: 0...1)
    var y: Double = Double.random(in: -0.2...1.0)
    let speed: Double = Double.random(in: 0.1...0.4)
    let length: CGFloat = CGFloat.random(in: 8...24)
    let opacity: Double = Double.random(in: 0.1...0.4)
}

// MARK: - Logs View
struct LogsView: View {
    let logs: [String]
    @Environment(\.dismiss) var dismiss
    
    var body: some View {
        NavigationView {
            ScrollView {
                VStack(alignment: .leading, spacing: 2) {
                    ForEach(logs.suffix(100), id: \.self) { line in
                        Text(line)
                            .font(.system(size: 10, design: .monospaced))
                            .foregroundColor(logColor(for: line))
                            .padding(.horizontal, 8)
                    }
                }
                .padding(.vertical, 8)
            }
            .background(Color(hex: "#0a0a0f"))
            .navigationTitle("Логи")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button("Готово") { dismiss() }
                        .foregroundColor(Color(hex: "#34d399"))
                }
                ToolbarItem(placement: .navigationBarLeading) {
                    Button {
                        UIPasteboard.general.string = logs.joined(separator: "\n")
                    } label: {
                        Image(systemName: "doc.on.doc")
                            .foregroundColor(Color(hex: "#34d399"))
                    }
                }
            }
        }
        .preferredColorScheme(.dark)
    }
    
    private func logColor(for line: String) -> Color {
        if line.contains("[error]") { return Color(hex: "#f87171") }
        if line.contains("[warn]") { return Color(hex: "#fbbf24") }
        if line.contains("INFO") { return Color(hex: "#60a5fa") }
        return Color(hex: "#9ca3af")
    }
}

// MARK: - Diagnostics View
struct DiagnosticsView: View {
    let diagnostics: DiagStatus
    @Environment(\.dismiss) var dismiss
    
    var body: some View {
        NavigationView {
            List {
                Section("Состояние") {
                    DiagnosticRow(label: "Статус", value: diagnostics.state, color: statusColor)
                    DiagnosticRow(label: "Категория", value: diagnostics.category, color: categoryColor)
                    DiagnosticRow(label: "Описание", value: diagnostics.explanation, color: .white)
                }
                
                Section("Счётчики") {
                    DiagnosticRow(label: "Ошибки подряд", value: "\(diagnostics.failCount)", color: Color(hex: "#f87171"))
                    DiagnosticRow(label: "Успехи подряд", value: "\(diagnostics.okCount)", color: Color(hex: "#34d399"))
                }
                
                if !diagnostics.lastError.isEmpty {
                    Section("Последняя ошибка") {
                        Text(diagnostics.lastError)
                            .font(.system(size: 11, design: .monospaced))
                            .foregroundColor(Color(hex: "#f87171"))
                    }
                }
            }
            .listStyle(.insetGrouped)
            .background(Color.black)
            .navigationTitle("Диагностика")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button("Готово") { dismiss() }
                        .foregroundColor(Color(hex: "#34d399"))
                }
            }
        }
        .preferredColorScheme(.dark)
    }
    
    private var statusColor: Color {
        switch diagnostics.state {
        case "HEALTHY": return Color(hex: "#34d399")
        case "RECOVERING": return Color(hex: "#fbbf24")
        case "FAILED": return Color(hex: "#f87171")
        default: return Color(hex: "#6b7280")
        }
    }
    
    private var categoryColor: Color {
        switch diagnostics.category {
        case "healthy": return Color(hex: "#34d399")
        case "network_down", "server_down", "dns_failure", "tls_failure", "server_blocked": return Color(hex: "#f87171")
        case "degraded": return Color(hex: "#fbbf24")
        default: return Color(hex: "#6b7280")
        }
    }
}

struct DiagnosticRow: View {
    let label: String
    let value: String
    let color: Color
    
    var body: some View {
        HStack {
            Text(label)
                .font(.system(size: 14))
                .foregroundColor(Color(hex: "#9ca3af"))
            Spacer()
            Text(value)
                .font(.system(size: 14, weight: .semibold, design: .monospaced))
                .foregroundColor(color)
        }
        .padding(.vertical, 4)
    }
}

// MARK: - Color Extension
extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a, r, g, b: UInt64
        switch hex.count {
        case 3:
            (a, r, g, b) = (255, (int >> 8) * 17, (int >> 4 & 0xF) * 17, (int & 0xF) * 17)
        case 6:
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        case 8:
            (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (1, 1, 1, 0)
        }
        self.init(
            .sRGB,
            red: Double(r) / 255,
            green: Double(g) / 255,
            blue: Double(b) / 255,
            opacity: Double(a) / 255
        )
    }
}

// MARK: - Press Events Modifier
struct PressEventsModifier: ViewModifier {
    var onPress: () -> Void
    var onRelease: () -> Void
    
    func body(content: Content) -> some View {
        content
            .simultaneousGesture(
                DragGesture(minimumDistance: 0)
                    .onChanged { _ in onPress() }
                    .onEnded { _ in onRelease() }
            )
    }
}

extension View {
    func pressEvents(onPress: @escaping () -> Void, onRelease: @escaping () -> Void = {}) -> some View {
        modifier(PressEventsModifier(onPress: onPress, onRelease: onRelease))
    }
}

// MARK: - Data Models
struct VPNStatus: Codable {
    var state: String = "stopped"
    var configId: String = ""
    var message: String = ""
    var connected: Bool = false
}

struct DiagStatus: Codable {
    var state: String = "HEALTHY"
    var category: String = "healthy"
    var explanation: String = "Всё работает нормально"
    var lastError: String = ""
    var failCount: Int = 0
    var okCount: Int = 0
}
