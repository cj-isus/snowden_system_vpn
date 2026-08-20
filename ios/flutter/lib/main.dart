import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter/services.dart' show Clipboard;

void main() {
  runApp(const SnowdenApp());
}

class SnowdenApp extends StatelessWidget {
  const SnowdenApp({super.key});
  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'snowden.system',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        brightness: Brightness.dark,
        scaffoldBackgroundColor: const Color(0xFF080808),
        colorScheme: const ColorScheme.dark(
          primary: Color(0xFF3CFF5A),
          surface: Color(0xFF101010),
        ),
        useMaterial3: true,
      ),
      home: const HomeScreen(),
    );
  }
}

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});
  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen>
    with SingleTickerProviderStateMixin {
  static const _channel = MethodChannel('snowden.system/vpn');
  static const _server = String.fromEnvironment('SNOWDEN_VPS_IP');
  static const _hy2Password = String.fromEnvironment('SNOWDEN_HY2_PASSWORD');
  static const _domain = String.fromEnvironment('SNOWDEN_VPN_DOMAIN');

  bool _connected = false;
  bool _connecting = false;
  late AnimationController _pulseController;

  // Logs
  final List<String> _logs = [];
  final ScrollController _logScroll = ScrollController();
  bool _showLogs = false;

  @override
  void initState() {
    super.initState();
    _pulseController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 2000),
    );
    _pulseController.repeat();

    // Listen for logs from Kotlin
    _channel.setMethodCallHandler((call) async {
      if (call.method == 'onLog') {
        final line = call.arguments as String? ?? '';
        if (mounted) {
          setState(() {
            _logs.add(line);
            if (_logs.length > 500) _logs.removeAt(0);
          });
          _scrollToBottom();
        }
      }
    });

    _checkStatus();
  }

  void _scrollToBottom() {
    if (_logScroll.hasClients) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _logScroll.animateTo(
          _logScroll.position.maxScrollExtent,
          duration: const Duration(milliseconds: 200),
          curve: Curves.easeOut,
        );
      });
    }
  }

  @override
  void dispose() {
    _pulseController.dispose();
    _logScroll.dispose();
    super.dispose();
  }

  void _checkStatus() async {
    try {
      final running = await _channel.invokeMethod<bool>('getStatus') ?? false;
      if (mounted) setState(() => _connected = running);
    } catch (_) {}
  }

  void _addLog(String line) {
    final timestamp = DateTime.now().toIso8601String().substring(11, 19);
    setState(() {
      _logs.add('[$timestamp] $line');
      if (_logs.length > 500) _logs.removeAt(0);
    });
    _scrollToBottom();
  }

  void _toggleVPN() async {
    if (_connecting) return;
    setState(() => _connecting = true);
    _addLog('>>> запуск VPN...');

    try {
      if (_connected) {
        _addLog('>>> отключение...');
        await _channel.invokeMethod('stopVpn');
        if (mounted) setState(() => _connected = false);
        _addLog('>>> VPN отключён');
      } else {
        final config = _buildConfig();
        _addLog('>>> конфиг создан (${config.length} байт)');
        await _channel.invokeMethod('startVpn', {'config': config});
        _addLog('>>> команда отправлена, ждём...');

        await Future.delayed(const Duration(seconds: 3));
        final running = await _channel.invokeMethod<bool>('getStatus') ?? false;
        if (mounted) setState(() => _connected = running);
        if (running) {
          _addLog('>>> VPN ЗАПУЩЕН ✓');
        } else {
          _addLog('>>> VPN НЕ ЗАПУСТИЛСЯ — смотрите logcat');
        }
      }
    } catch (e) {
      _addLog('!!! ОШИБКА: $e');
    } finally {
      if (mounted) setState(() => _connecting = false);
    }
  }

  String _buildConfig() {
    return jsonEncode({
      "log": {"level": "debug"},
      "dns": {
        "servers": [
          {"type": "https", "tag": "remote", "server": "1.1.1.1", "detour": "proxy"},
          {"type": "local", "tag": "local", "detour": "direct"}
        ],
        "rules": [
          {"outbound": "any", "server": "local"}
        ],
        "final": "remote",
        "strategy": "ipv4_only"
      },
      "inbounds": [
        {"type": "tun", "tag": "tun-in", "address": ["172.19.0.1/30"], "mtu": 1500, "auto_route": false, "strict_route": false, "stack": "system"}
      ],
      "outbounds": [
        {
          "type": "selector",
          "tag": "proxy",
          "outbounds": ["hysteria2"],
          "default": "hysteria2"
        },
        {
          "type": "hysteria2",
          "tag": "hysteria2",
          "server": _server,
          "server_port": 8443,
          "password": _hy2Password,
          "tls": {
            "enabled": true,
            "server_name": _domain,
            "alpn": ["h3"]
          }
        },
        {"type": "direct", "tag": "direct"},
        {"type": "block", "tag": "block"}
      ],
      "route": {
        "rules": [
          {"action": "sniff"},
          {"protocol": "dns", "action": "hijack-dns"},
          {"domain_suffix": [".ru", ".su", ".рф"], "action": "direct"},
          {"domain": [
            "yandex.ru", "ya.ru", "vk.com", "mail.ru", "ok.ru", "avito.ru",
            "ozon.ru", "wildberries.ru", "gosuslugi.ru", "esia.gosuslugi.ru",
            "sberbank.ru", "sber.ru", "online.sberbank.ru",
            "tinkoff.ru", "tbank.ru", "security.tinkoff.ru",
            "vtb.ru", "bank-vtb.ru",
            "alfabank.ru", "online.alfabank.ru",
            "rshb.ru", "gazprombank.ru", "mkb.ru",
            "pochta.ru", "2gis.ru", "hh.ru", "kinopoisk.ru",
            "rambler.ru", "lenta.ru", "rbc.ru", "rt.ru",
            "mts.ru", "beeline.ru", "megafon.ru", "tele2.ru",
            "qiwi.com", "yoomoney.ru",
            "drom.ru", "cian.ru",
            "ivi.ru", "okko.ru", "megogo.net",
            "mvideo.ru", "dns-shop.ru", "eldorado.ru", "citilink.ru",
            "sportmaster.ru", "lamoda.ru",
            "passport.yandex.ru", "oauth.yandex.ru"
          ], "action": "direct"},
          {"domain_suffix": [
            "googlevideo.com", "ytimg.com", "youtube.com", "youtu.be",
            "googleapis.com", "gstatic.com",
            "discord.com", "discordapp.com", "discord.gg",
            "openai.com", "anthropic.com", "claude.ai", "x.ai",
            "twitter.com", "x.com", "facebook.com", "instagram.com",
            "whatsapp.com", "reddit.com",
            "twitch.tv", "netflix.com", "spotify.com",
            "steam.com", "steampowered.com", "steamcommunity.com"
          ], "outbound": "proxy"},
          {"domain": ["t.me", "telegram.org"], "outbound": "proxy"},
          {"ip_cidr": [
            "91.108.0.0/16", "149.154.0.0/16",
            "185.76.151.0/24"
          ], "outbound": "proxy"},
          {"ip_is_private": true, "action": "direct"}
        ],
        "final": "proxy",
        "default_domain_resolver": "local",
        "auto_detect_interface": true
      }
    });
  }

  void _copyLogs() async {
    final text = _logs.join('\n');
    await Clipboard.setData(ClipboardData(text: text));
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Логи скопированы'), duration: Duration(seconds: 1)),
      );
    }
  }

  void _clearLogs() {
    setState(() => _logs.clear());
  }

  @override
  Widget build(BuildContext context) {
    final screenW = MediaQuery.of(context).size.width;
    final isSmall = screenW < 400;

    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            // Top bar
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 16, 12, 8),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const Text(
                    'snowden.system',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                      color: Color(0xFF3CFF5A),
                    ),
                  ),
                  Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                        decoration: BoxDecoration(
                          border: Border.all(color: _statusColor.withOpacity(0.3)),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Container(
                              width: 8, height: 8,
                              decoration: BoxDecoration(
                                color: _statusColor,
                                shape: BoxShape.circle,
                                boxShadow: _connected ? [BoxShadow(color: _statusColor, blurRadius: 8)] : null,
                              ),
                            ),
                            const SizedBox(width: 6),
                            Text(_statusText, style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: _statusColor)),
                          ],
                        ),
                      ),
                      const SizedBox(width: 8),
                      // Logs toggle button
                      GestureDetector(
                        onTap: () => setState(() => _showLogs = !_showLogs),
                        child: Container(
                          padding: const EdgeInsets.all(6),
                          decoration: BoxDecoration(
                            border: Border.all(color: Colors.grey.shade700),
                            borderRadius: BorderRadius.circular(6),
                          ),
                          child: Icon(
                            _showLogs ? Icons.terminal : Icons.terminal_outlined,
                            size: 16,
                            color: _showLogs ? const Color(0xFF3CFF5A) : Colors.grey.shade600,
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),

            // Main content
            Expanded(
              child: _showLogs ? _buildLogPanel() : _buildVPNPanel(isSmall),
            ),

            // Bottom
            Padding(
              padding: const EdgeInsets.only(bottom: 16),
              child: Text('snowden.system v1.0.0', style: TextStyle(fontSize: 10, color: Colors.grey.shade700)),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildVPNPanel(bool isSmall) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            width: isSmall ? 60 : 72, height: isSmall ? 60 : 72,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(18),
              border: Border.all(color: const Color(0xFF3CFF5A).withOpacity(0.2)),
            ),
            child: const Icon(Icons.shield_outlined, size: 36, color: Color(0xFF3CFF5A)),
          ),
          const SizedBox(height: 16),
          Text(
            _connected ? 'ЗАЩИЩЁН' : 'НЕ ЗАЩИЩЁН',
            style: TextStyle(fontSize: isSmall ? 20 : 24, fontWeight: FontWeight.w700, color: _statusColor, letterSpacing: 2),
          ),
          const SizedBox(height: 4),
          Text(
            _connected ? 'VPS Netherlands · авто' : 'privacy is a human right',
            style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
          ),
          const SizedBox(height: 48),
          GestureDetector(
            onTap: _toggleVPN,
            child: AnimatedBuilder(
              animation: _pulseController,
              builder: (context, _) {
                final pulse = _connected ? (0.8 + 0.2 * _pulseController.value) : 1.0;
                return Container(
                  width: isSmall ? 120 : 140, height: isSmall ? 120 : 140,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    border: Border.all(color: _connected ? const Color(0xFF3CFF5A) : const Color(0xFF2A2A35), width: 2),
                    boxShadow: _connected ? [BoxShadow(color: const Color(0xFF3CFF5A).withOpacity(0.15 * pulse), blurRadius: 40 * pulse, spreadRadius: 2)] : null,
                  ),
                  child: _connecting
                      ? const Center(child: SizedBox(width: 32, height: 32, child: CircularProgressIndicator(color: Color(0xFF3CFF5A), strokeWidth: 2)))
                      : Icon(Icons.power_settings_new, size: isSmall ? 40 : 48, color: _connected ? const Color(0xFF3CFF5A) : Colors.grey.shade500),
                );
              },
            ),
          ),
          const SizedBox(height: 16),
          Text(
            _connecting ? 'ПОДКЛЮЧЕНИЕ...' : _connected ? 'ВЫКЛЮЧИТЬ' : 'ПОДКЛЮЧИТЬ',
            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: Colors.grey.shade500, letterSpacing: 1),
          ),
        ],
      ),
    );
  }

  Widget _buildLogPanel() {
    return Column(
      children: [
        // Toolbar
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
          child: Row(
            children: [
              Text(
                'ЛОГИ (${_logs.length})',
                style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: Colors.grey, letterSpacing: 1),
              ),
              const Spacer(),
              GestureDetector(
                onTap: _copyLogs,
                child: const Padding(
                  padding: EdgeInsets.all(6),
                  child: Icon(Icons.copy, size: 16, color: Color(0xFF3CFF5A)),
                ),
              ),
              GestureDetector(
                onTap: _clearLogs,
                child: Padding(
                  padding: const EdgeInsets.all(6),
                  child: Icon(Icons.delete_outline, size: 16, color: Colors.grey.shade600),
                ),
              ),
            ],
          ),
        ),
        // Terminal
        Expanded(
          child: Container(
            margin: const EdgeInsets.symmetric(horizontal: 8),
            decoration: BoxDecoration(
              color: const Color(0xFF050505),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: const Color(0xFF3CFF5A).withOpacity(0.05)),
            ),
            child: ListView.builder(
              controller: _logScroll,
              padding: const EdgeInsets.all(8),
              itemCount: _logs.length,
              itemBuilder: (context, index) {
                final line = _logs[index];
                Color color = const Color(0xFF44FF66);
                if (line.contains('!!!') || line.contains('ERROR') || line.contains('FAIL')) {
                  color = const Color(0xFFFF4D4D);
                } else if (line.contains('WARN')) {
                  color = const Color(0xFFFFC533);
                }
                return Text(
                  line,
                  style: TextStyle(
                    fontFamily: 'monospace',
                    fontSize: 11,
                    height: 1.4,
                    color: color,
                  ),
                );
              },
            ),
          ),
        ),
      ],
    );
  }

  Color get _statusColor {
    if (_connecting) return const Color(0xFFFFC533);
    if (_connected) return const Color(0xFF3CFF5A);
    return Colors.grey.shade600;
  }

  String get _statusText {
    if (_connecting) return 'ПОДКЛЮЧЕНИЕ';
    if (_connected) return 'ЗАЩИЩЁН';
    return 'ОТКЛЮЧЕН';
  }
}
