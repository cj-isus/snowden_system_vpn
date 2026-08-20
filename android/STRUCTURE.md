# STRUCTURE.md — `android/`

Android is a separate Flutter + Kotlin VPN client. It does not share the Windows
Go process; it uses the pinned `libbox.aar` runtime through `SnowdenVpnService`.

## Components

| Path | Role |
|---|---|
| `lib/main.dart` | Flutter UI, local build-time config injection and protected TUN JSON |
| `android/app/src/main/kotlin/.../MainActivity.kt` | Flutter MethodChannel bridge |
| `.../SnowdenPlatformInterface.kt` | platform bridge and service commands |
| `.../SnowdenVpnService.kt` | Android VPNService lifecycle and libbox start/stop |
| `android/app/libs/libbox.aar` | local pinned sing-box Android runtime; do not commit if supplied separately |
| `config.example.json` | non-secret `--dart-define-from-file` schema |
| `config.local.json` | ignored local credentials; never publish |
| `build_android.bat` | checkout-relative build helper with pinned AAR URL |
| `ANDROID_SETUP.md` | ADB and local acceptance checklist |

## Runtime graph

```text
Flutter button
  → MethodChannel("com.snowden.system/vpn")
  → SnowdenVpnService
  → libbox TUN
  → selector "proxy" → provisioned protected Hysteria2 channel
```

The Android MVP intentionally uses only a channel whose credentials are present
in the local profile. A missing profile refuses to start; it does not launch a
partial config or silently use `direct`. Full adaptive controller and persistent
ChannelMemory remain a later mobile phase.

## Local build

```powershell
cd D:\snowden-v2\android
flutter doctor -v
flutter pub get
flutter build apk --release --dart-define-from-file=config.local.json
adb devices
adb install -r .\build\app\outputs\flutter-apk\app-release.apk
```

The connected device must show `device`, not `unauthorized` or `offline`. Live
acceptance covers VPN permission, foreground notification, TUN start, protected
HTTPS traffic and clean stop.
