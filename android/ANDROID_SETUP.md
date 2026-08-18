# Android VPN Integration Guide

## Step 1: Download libbox.aar

Download the pre-built AAR from GitHub Releases:

**URL:** https://github.com/Leadaxe/sing-box-lx/releases/download/v1.14.0-lx.3/libbox-1.14.0-lx.3.aar

**Save to:** `D:\ОБХОДЫ\snowden_android\android\app\libs\libbox.aar`

**File size:** ~98 MB

## Step 2: Build APK

```bash
cd D:\ОБХОДЫ\snowden_android
flutter build apk --release
```

## Step 3: Install on Android

```bash
adb install build\app\outputs\flutter-apk\app-release.apk
```
