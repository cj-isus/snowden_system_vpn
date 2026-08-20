import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

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
  static const MethodChannel _vpnChannel = MethodChannel('com.snowden.system/vpn');

  bool _connected = false;
  bool _connecting = false;
  String _statusText = 'ОТКЛЮЧЕН';
  String _subStatus = 'privacy is a human right';
  late AnimationController _pulseController;
  Timer? _statusTimer;

  // Runtime credentials are injected only at local build time with
  // --dart-define-from-file=android/config.local.json. No server IP, UUID or
  // password belongs in the repository or in the default binary.
  static const String _server = String.fromEnvironment('SNOWDEN_VPS_IP');
  static const String _password = String.fromEnvironment('SNOWDEN_HY2_PASSWORD');
  static const String _domain = String.fromEnvironment('SNOWDEN_VPN_DOMAIN');

  bool get _configReady =>
      _server.isNotEmpty && _password.isNotEmpty && _domain.isNotEmpty;

  // Android VPN config — selector-owned TUN route. Protected traffic never
  // falls back to direct; only explicit RU/private rules use direct.
  String get _config => jsonEncode({
    "log": {"level": "info", "timestamp": true},
    "dns": {
      "servers": [
        {"type": "https", "tag": "cloudflare", "server": "1.1.1.1", "path": "/dns-query", "detour": "proxy"},
        {"type": "local", "tag": "local", "detour": "direct"}
      ],
      "rules": [{"outbound": "any", "server": "local"}],
      "strategy": "ipv4_only"
    },
    "inbounds": [
      {
        "type": "tun",
        "tag": "tun-in",
        "inet4_address": "172.19.0.1/30",
        "inet6_address": "fdfe:dcba:9876::1/126",
        "mtu": 9000,
        "auto_route": true,
        "strict_route": false,
        "stack": "system",
        "sniff": true
      }
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
        "password": _password,
        "tls": {
          "enabled": true,
          "server_name": _domain
        }
      },
      {"type": "direct", "tag": "direct"},
      {"type": "block", "tag": "block"}
    ],
    "route": {
      "rules": [
        {"action": "sniff"},
        {"action": "hijack-dns", "inbound": "tun-in", "protocol": "dns"},
        {"domain_suffix": [".ru", ".su", ".рф"], "action": "direct"},
        {"domain": ["yandex.ru", "vk.com", "mail.ru", "sberbank.ru", "tinkoff.ru", "gosuslugi.ru"], "action": "direct"},
        {"domain_suffix": ["googlevideo.com", "youtube.com", "youtu.be"], "outbound": "proxy"},
        {"domain": ["t.me", "telegram.org"], "outbound": "proxy"},
        {"domain_suffix": ["discord.com", "discord.gg"], "outbound": "proxy"},
        {"domain_suffix": ["twitter.com", "x.com", "instagram.com"], "outbound": "proxy"},
        {"ip_is_private": true, "action": "direct"}
      ],
      "final": "proxy",
      "auto_detect_interface": true
    }
  });

  @override
  void initState() {
    super.initState();
    _pulseController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 2000),
    );
    _pulseController.repeat();
    _startStatusPolling();
  }

  @override
  void dispose() {
    _pulseController.dispose();
    _statusTimer?.cancel();
    super.dispose();
  }

  void _startStatusPolling() {
    _statusTimer = Timer.periodic(const Duration(seconds: 2), (_) async {
      try {
        final bool running = await _vpnChannel.invokeMethod('getStatus');
        if (running != _connected) {
          setState(() {
            _connected = running;
            _updateStatus();
          });
        }
      } catch (e) {
        // Channel not ready yet
      }
    });
  }

  void _updateStatus() {
    if (_connected) {
      _statusText = 'ЗАЩИЩЁН';
      _subStatus = 'VPS Netherlands · 45ms';
    } else {
      _statusText = 'НЕ ЗАЩИЩЁН';
      _subStatus = 'privacy is a human right';
    }
  }

  Future<void> _toggleVPN() async {
    if (_connecting) return;
    if (!_configReady) {
      setState(() {
        _subStatus = 'Нет локального профиля: соберите с config.local.json';
      });
      return;
    }
    setState(() => _connecting = true);

    try {
      if (_connected) {
        await _vpnChannel.invokeMethod('stopVpn');
        setState(() {
          _connected = false;
          _connecting = false;
        });
        _updateStatus();
      } else {
        await _vpnChannel.invokeMethod('startVpn', {'config': _config});
        setState(() {
          _connected = true;
          _connecting = false;
        });
        _updateStatus();
        HapticFeedback.mediumImpact();
      }
    } on PlatformException catch (e) {
      setState(() {
        _connecting = false;
        _subStatus = 'Ошибка: ${e.message}';
      });
    }
  }

  Color get _statusColor {
    if (_connecting) return const Color(0xFFFFC533);
    if (_connected) return const Color(0xFF3CFF5A);
    return Colors.grey.shade600;
  }

  @override
  Widget build(BuildContext context) {
    final screenW = MediaQuery.of(context).size.width;
    final isSmall = screenW < 400;

    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            // ─── Top bar ───
            Padding(
              padding: const EdgeInsets.all(20),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const Text(
                    'snowden.system',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                      color: Color(0xFF3CFF5A),
                      letterSpacing: -0.5,
                    ),
                  ),
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
                          width: 8,
                          height: 8,
                          decoration: BoxDecoration(
                            color: _statusColor,
                            shape: BoxShape.circle,
                            boxShadow: _connected
                                ? [BoxShadow(color: _statusColor, blurRadius: 8)]
                                : null,
                          ),
                        ),
                        const SizedBox(width: 6),
                        Text(
                          _statusText,
                          style: TextStyle(
                            fontSize: 11,
                            fontWeight: FontWeight.w600,
                            color: _statusColor,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),

            // ─── Center ───
            Expanded(
              child: Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    // Logo
                    Container(
                      width: isSmall ? 60 : 72,
                      height: isSmall ? 60 : 72,
                      decoration: BoxDecoration(
                        borderRadius: BorderRadius.circular(18),
                        border: Border.all(
                          color: const Color(0xFF3CFF5A).withOpacity(0.2),
                        ),
                      ),
                      child: const Icon(
                        Icons.shield_outlined,
                        size: 36,
                        color: Color(0xFF3CFF5A),
                      ),
                    ),
                    const SizedBox(height: 16),

                    // Status
                    Text(
                      _statusText,
                      style: TextStyle(
                        fontSize: isSmall ? 20 : 24,
                        fontWeight: FontWeight.w700,
                        color: _statusColor,
                        letterSpacing: 2,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      _subStatus,
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade600,
                      ),
                    ),
                    const SizedBox(height: 48),

                    // Power button
                    GestureDetector(
                      onTap: _toggleVPN,
                      child: AnimatedBuilder(
                        animation: _pulseController,
                        builder: (context, _) {
                          final pulse = _connected
                              ? (0.8 + 0.2 * _pulseController.value)
                              : 1.0;
                          return Container(
                            width: isSmall ? 120 : 140,
                            height: isSmall ? 120 : 140,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              border: Border.all(
                                color: _connected
                                    ? const Color(0xFF3CFF5A)
                                    : const Color(0xFF2A2A35),
                                width: 2,
                              ),
                              boxShadow: _connected
                                  ? [
                                      BoxShadow(
                                        color: const Color(0xFF3CFF5A)
                                            .withOpacity(0.15 * pulse),
                                        blurRadius: 40 * pulse,
                                        spreadRadius: 2,
                                      )
                                    ]
                                  : null,
                            ),
                            child: _connecting
                                ? const Center(
                                    child: SizedBox(
                                      width: 32,
                                      height: 32,
                                      child: CircularProgressIndicator(
                                        color: Color(0xFF3CFF5A),
                                        strokeWidth: 2,
                                      ),
                                    ),
                                  )
                                : Icon(
                                    Icons.power_settings_new,
                                    size: isSmall ? 40 : 48,
                                    color: _connected
                                        ? const Color(0xFF3CFF5A)
                                        : Colors.grey.shade500,
                                  ),
                          );
                        },
                      ),
                    ),
                    const SizedBox(height: 16),
                    Text(
                      _connecting
                          ? 'ПОДКЛЮЧЕНИЕ...'
                          : _connected
                              ? 'ВЫКЛЮЧИТЬ'
                              : 'ПОДКЛЮЧИТЬ',
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: Colors.grey.shade500,
                        letterSpacing: 1,
                      ),
                    ),
                  ],
                ),
              ),
            ),

            // ─── Bottom stats ───
            if (_connected)
              Padding(
                padding: const EdgeInsets.only(bottom: 12, left: 20, right: 20),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                  decoration: BoxDecoration(
                    color: const Color(0xFF101010),
                    borderRadius: BorderRadius.circular(14),
                    border: Border.all(
                      color: const Color(0xFF3CFF5A).withOpacity(0.08),
                    ),
                  ),
                  child: const Row(
                    mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                    children: [
                      _StatItem(label: '↓', value: '2.3', unit: 'МБ/с'),
                      _Divider(),
                      _StatItem(label: '↑', value: '0.4', unit: 'МБ/с'),
                      _Divider(),
                      _StatItem(label: '⏱', value: '02:47', unit: ''),
                    ],
                  ),
                ),
              ),
            Padding(
              padding: const EdgeInsets.only(bottom: 20),
              child: Text(
                'snowden.system v1.0.0',
                style: TextStyle(
                  fontSize: 10,
                  color: Colors.grey.shade700,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _StatItem extends StatelessWidget {
  final String label;
  final String value;
  final String unit;
  const _StatItem({required this.label, required this.value, required this.unit});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.baseline,
      textBaseline: TextBaseline.alphabetic,
      children: [
        Text(label,
            style: const TextStyle(
                fontSize: 14, fontWeight: FontWeight.w700, color: Color(0xFF3CFF5A))),
        const SizedBox(width: 4),
        Text(value,
            style: const TextStyle(
                fontSize: 16, fontWeight: FontWeight.w700, color: Colors.white)),
        if (unit.isNotEmpty)
          Text(' $unit', style: TextStyle(fontSize: 10, color: Colors.grey.shade600)),
      ],
    );
  }
}

class _Divider extends StatelessWidget {
  const _Divider();

  @override
  Widget build(BuildContext context) {
    return Container(width: 1, height: 24, color: Colors.grey.shade800);
  }
}
