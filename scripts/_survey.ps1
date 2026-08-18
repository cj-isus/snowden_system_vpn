$ErrorActionPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding           = [System.Text.Encoding]::UTF8

Write-Host "===== ALL DRIVES (free space for backup target) ====="
Get-Volume |
    Where-Object { $_.DriveLetter } |
    Select-Object DriveLetter, FileSystemLabel, FileSystem, DriveType,
                  @{N='Size(GB)';   E={[math]::Round($_.Size/1GB,2)}},
                  @{N='Free(GB)';   E={[math]::Round($_.SizeRemaining/1GB,2)}} |
    Sort-Object DriveLetter |
    Format-Table -AutoSize

Write-Host "===== PHYSICAL DISKS (is Toshiba an HDD or SSD?) ====="
Get-PhysicalDisk |
    Select-Object DeviceId, FriendlyName, MediaType, BusType, SpindleSaved,
                  @{N='Size(GB)';E={[math]::Round($_.Size/1GB,2)}},
                  HealthStatus, OperationalStatus |
    Format-Table -AutoSize

Write-Host "===== SMART (detailed) via Get-PhysicalDisk - GetStorageReliabilityCounter ====="
Get-PhysicalDisk | ForEach-Object {
    $rc = $_ | Get-StorageReliabilityCounter
    [PSCustomObject]@{
        Device       = $_.DeviceId
        Name         = $_.FriendlyName
        TempC        = $rc.Temperature
        ReadErrors   = $rc.ReadErrorsTotal
        WriteErrors  = $rc.WriteErrorsTotal
        Wear         = $rc.Wear
        PowerOnHours = $rc.PowerOnHours
    }
} | Format-Table -AutoSize
