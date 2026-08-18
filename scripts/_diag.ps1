$ErrorActionPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding           = [System.Text.Encoding]::UTF8

Write-Host "===== 1. PARTITION / VOLUME -> DISK MAPPING ====="
Get-Partition -DriveLetter E |
    Select-Object DiskNumber, PartitionNumber, DriveLetter,
                  @{N='Size(GB)';E={[math]::Round($_.Size/1GB,2)}} |
    Format-List

Write-Host "===== 2. PHYSICAL DISK ====="
Get-PhysicalDisk |
    Select-Object DeviceId, FriendlyName, MediaType, BusType,
                  @{N='Size(GB)';E={[math]::Round($_.Size/1GB,2)}},
                  HealthStatus, OperationalStatus,
                  @{N='MediaType';E={$_.MediaType}} |
    Format-Table -AutoSize

# Which physical disk hosts E:
$diskNum = (Get-Partition -DriveLetter E).DiskNumber
Write-Host "Physical disk hosting E: -> DiskNumber = $diskNum"
Get-PhysicalDisk | Where-Object DeviceId -eq $diskNum |
    Select-Object DeviceId, FriendlyName, MediaType, BusType, FirmwareVersion,
                  @{N='Size(GB)';E={[math]::Round($_.Size/1GB,2)}},
                  HealthStatus, OperationalStatus |
    Format-List

Write-Host "===== 3. STORAGE RELIABILITY / RELIABILITY COUNTERS ====="
Get-PhysicalDisk | Where-Object DeviceId -eq $diskNum |
    Get-StorageReliabilityCounter |
    Select-Object Temperature, ReadErrorsTotal, WriteErrorsTotal,
                  Wear, PowerOnHours, StartStopCycleCount |
    Format-List

Write-Host "===== 4. DIRTY BIT / VOLUME STATE ====="
fsutil dirty query E:

Write-Host "===== 5. chkdsk (READ-ONLY) ====="
chkdsk E: 2>&1 | Out-String
