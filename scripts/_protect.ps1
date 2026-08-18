$ErrorActionPreference = 'Continue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding           = [System.Text.Encoding]::UTF8

Write-Host "===== CURRENT DEFRAG SCHEDULE ====="
# Windows storage optimization / scheduled defrag state
Get-PSDrive -Name E | Format-List Name, Root

Write-Host "--- defrag volume info (current optimization status) ---"
defrag E: /O /U /V 2>&1 | Out-String

Write-Host ""
Write-Host "===== INDEXING (Windows Search) status on E: ====="
$svc = Get-Service WSearch -ErrorAction SilentlyContinue
"Windows Search service: " + $(if($svc){$svc.Status}else{'not present'})
# Check if E: is indexed via registry IndexerLocations (best-effort)
$reg = "HKLM:\SOFTWARE\Microsoft\Windows Search\Gather\Windows\SystemIndex\Sites\Local"
Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows Search" -ErrorAction SilentlyContinue |
    Select-Object SetupCompletedSuccessfully | Format-List
