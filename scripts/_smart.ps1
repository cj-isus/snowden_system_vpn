$ErrorActionPreference = 'Continue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding           = [System.Text.Encoding]::UTF8
$sc = "C:\Program Files\smartmontools\bin\smartctl.exe"

Write-Host "===== 1. DEVICE SCAN (how smartctl sees disks) ====="
& $sc --scan 2>&1
Write-Host ""
& $sc --scan-open 2>&1

Write-Host ""
Write-Host "===== 2. INFO + SAT passthrough for E: ====="
# /dev/sdX notation: E is disk 1 -> /dev/sdb on smartctl usually.
# Safer: use the drive letter form with sat (SAT layer for USB).
& $sc -a -d sat "E:" 2>&1
