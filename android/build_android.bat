@echo off
chcp 65001 >nul
echo ==========================================
echo  snowden.system - Android Build Script
echo ==========================================
echo.

set PROJECT_DIR=%~dp0
set AAR_URL=https://github.com/Leadaxe/sing-box-lx/releases/download/v1.14.0-lx.3/libbox-1.14.0-lx.3.aar
set AAR_PATH=%PROJECT_DIR%android\app\libs\libbox.aar

echo [1/4] Checking Flutter...
flutter --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: Flutter not found in PATH
    echo Please add Flutter to your PATH
    exit /b 1
)
echo OK

echo.
echo [2/4] Checking AAR library...
if not exist "%AAR_PATH%" (
    echo AAR not found. Downloading...
    if not exist "%PROJECT_DIR%android\app\libs" mkdir "%PROJECT_DIR%android\app\libs"
    
    echo Downloading from GitHub...
    echo URL: %AAR_URL%
    echo Target: %AAR_PATH%
    
    :: Try PowerShell download
    powershell -Command "& {try { Invoke-WebRequest -Uri '%AAR_URL%' -OutFile '%AAR_PATH%' -UseBasicParsing } catch { exit 1 }}"
    
    if not exist "%AAR_PATH%" (
        echo ERROR: Failed to download AAR
        echo Please download manually:
        echo %AAR_URL%
        echo And save to: %AAR_PATH%
        exit /b 1
    )
    echo Download complete!
) else (
    echo AAR already exists: %AAR_PATH%
)

echo.
echo [3/4] Checking local build profile...
if not exist "%PROJECT_DIR%config.local.json" (
    echo ERROR: config.local.json not found.
    echo Copy config.example.json to config.local.json and fill it locally.
    exit /b 1
)

echo.
echo [4/5] Getting Flutter dependencies...
cd /d "%PROJECT_DIR%"
flutter pub get
if errorlevel 1 (
    echo ERROR: flutter pub get failed
    exit /b 1
)

echo.
echo [5/5] Building APK with local profile...
flutter build apk --release --dart-define-from-file=config.local.json
if errorlevel 1 (
    echo ERROR: Build failed
    exit /b 1
)

echo.
echo ==========================================
echo  BUILD SUCCESS!
echo ==========================================
echo APK location:
echo %PROJECT_DIR%build\app\outputs\flutter-apk\app-release.apk
echo.
echo To install on device:
echo adb install "%PROJECT_DIR%build\app\outputs\flutter-apk\app-release.apk"
echo.
pause
