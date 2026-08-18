$ErrorActionPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding           = [System.Text.Encoding]::UTF8

Write-Host "===== VOLUME INFO ====="
Get-Volume -DriveLetter E |
    Select-Object DriveLetter,
                  FileSystemLabel,
                  FileSystem,
                  DriveType,
                  @{N='Size(GB)';   E={[math]::Round($_.Size/1GB,2)}},
                  @{N='Free(GB)';   E={[math]::Round($_.SizeRemaining/1GB,2)}},
                  @{N='Used(GB)';   E={[math]::Round(($_.Size-$_.SizeRemaining)/1GB,2)}},
                  @{N='Free(%)';    E={[math]::Round($_.SizeRemaining/$_.Size*100,1)}},
                  HealthStatus,
                  OperationalStatus |
    Format-List

Write-Host "===== PSDRIVE (mounted letter) ====="
Get-PSDrive E |
    Select-Object Name,
                  Root,
                  DisplayRoot,
                  Provider,
                  @{N='Used(GB)'; E={[math]::Round($_.Used/1GB,2)}},
                  @{N='Free(GB)'; E={[math]::Round($_.Free/1GB,2)}} |
    Format-List

Write-Host "===== TOP-LEVEL FOLDERS/FILES ====="
Get-ChildItem E:\ -Force |
    Select-Object Mode,
                  LastWriteTime,
                  @{N='Size(MB)'; E={if($_.PSIsContainer){''}else{[math]::Round($_.Length/1MB,2)}}},
                  Name |
    Format-Table -AutoSize

Write-Host "===== FOLDER SIZES (top 15) ====="
Get-ChildItem E:\ -Directory -Force |
    ForEach-Object {
        $size = (Get-ChildItem $_.FullName -Recurse -Force -File -ErrorAction SilentlyContinue |
                 Measure-Object -Property Length -Sum).Sum
        [PSCustomObject]@{
            Folder     = $_.Name
            'Size(GB)' = [math]::Round($size/1GB,2)
        }
    } |
    Sort-Object 'Size(GB)' -Descending |
    Select-Object -First 15 |
    Format-Table -AutoSize
