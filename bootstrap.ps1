param([string]$Version = $env:DEVICE_USAGE_VERSION,
 [ValidateSet('menu','install','repair','sources','status','uninstall')][string]$Mode = 'menu')
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Windows required' }
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($arch -eq 'x64') { $arch = 'amd64' }
if ($arch -notin @('amd64','arm64')) { throw 'Supported: Windows x64/arm64' }
if ([Console]::IsInputRedirected) { throw 'Run the downloaded script in an interactive PowerShell terminal' }
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory $tmp | Out-Null
function Fetch([string]$Url, [string]$Path, [long]$Limit) {
 & curl.exe --fail --silent --show-error --location --max-redirs 5 --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 20 --max-time 180 --max-filesize $Limit $Url -o $Path
 if ($LASTEXITCODE -ne 0) { throw 'Release download failed' }
 if ((Get-Item $Path).Length -gt $Limit) { throw 'Download too large' }
}
try {
 if (!$Version) {
  Fetch 'https://api.github.com/repos/NotRllyRn/cpa-device-usage-keeper/releases/latest' "$tmp\latest.json" 1048576
  $release = Get-Content -Raw "$tmp\latest.json" | ConvertFrom-Json
  if ($release.draft -isnot [bool] -or $release.draft -or $release.prerelease -isnot [bool] -or $release.prerelease -or $release.tag_name -isnot [string]) { throw 'Invalid stable release response' }
  $Version = $release.tag_name
 }
 if ($Version -cnotmatch '\Av[0-9]+\.[0-9]+\.[0-9]+\z') { throw 'Invalid stable release tag' }
 $lib = Join-Path $HOME '.local\lib\device-usage-keeper'
 $existing = (Test-Path "$HOME\.local\state\device-usage-keeper") -or (Test-Path "$HOME\.local\bin\device-sync.exe") -or (Test-Path $lib) -or (Get-ScheduledTask -TaskName DeviceUsageKeeper -ErrorAction SilentlyContinue)
 if ($existing) {
  $installed = 'unknown'
  $marker = Join-Path $lib 'release-version'
  if ((Test-Path $marker) -and (Get-Item $marker).Length -lt 128) {
   $saved = [IO.File]::ReadAllText($marker)
   if ($saved -cmatch '\Av[0-9]+\.[0-9]+\.[0-9]+\n?\z') { $installed = $saved.Trim() }
  }
  Write-Output "Installed: $installed; available: $Version"
  if ($installed -ne $Version) {
   # Deliberately do not route this through repair: repair is not transactional update.
   $action = 'switch from unknown version'
   if ($installed -ne 'unknown') {
    $oldParts = $installed.Substring(1).Split('.')
    $newParts = $Version.Substring(1).Split('.')
    $comparison = 0
    for ($i = 0; $i -lt 3 -and $comparison -eq 0; $i++) {
     $oldPart = $oldParts[$i].TrimStart('0'); $newPart = $newParts[$i].TrimStart('0')
     $comparison = $newPart.Length.CompareTo($oldPart.Length)
     if ($comparison -eq 0) { $comparison = [string]::CompareOrdinal($newPart, $oldPart) }
    }
    $action = if ($comparison -gt 0) { 'upgrade' } else { 'switch/downgrade' }
   }
   Write-Output 'Windows in-place upgrade/switch is unsupported; no files will be changed.'
   $answer = Read-Host "Confirm $action to ${Version}? [y/N]"
   if ($answer -in @('y','yes')) { throw 'Upgrade blocked: Windows needs a validated stopped-task backup/rollback update. Use a reviewed manual migration; private state must be retained.' }
   Write-Output 'Cancelled; installation unchanged.'
   return
  }
 }
 $asset = "device-usage-$Version-windows-$arch.zip"
 $base = "https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/download/$Version"
 Fetch "$base/$asset.sha256" "$tmp\checksum" 1024
 Fetch "$base/$asset" "$tmp\archive.zip" 134217728
 $checksum = [IO.File]::ReadAllText("$tmp\checksum")
 if ($checksum -cnotmatch ('\A([a-f0-9]{64})  ' + [regex]::Escape($asset) + '\n\z')) { throw 'Invalid checksum manifest' }
 if ((Get-FileHash "$tmp\archive.zip" -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Matches[1]) { throw 'Release checksum mismatch' }
 Add-Type -AssemblyName System.IO.Compression.FileSystem
 $zip = [IO.Compression.ZipFile]::OpenRead("$tmp\archive.zip")
 try {
  $expected = @('install.sh','install-macos.sh','install.ps1','LICENSE','BUILD.txt','RELEASE_VERSION','SHA256SUMS','collector.tgz',"device-sync-windows-$arch.exe")
  $seen = @{}; [long]$size = 0
  foreach ($entry in $zip.Entries) {
   if ($entry.FullName -cnotin $expected -or $seen.ContainsKey($entry.FullName)) { throw 'Unexpected archive member' }
   $kind = ($entry.ExternalAttributes -shr 16) -band 61440
   if ($kind -notin @(0,32768)) { throw 'Non-regular archive member' }
   $seen[$entry.FullName] = $true; $size += $entry.Length
  }
  if ($seen.Count -ne $expected.Count -or $size -gt 268435456) { throw 'Invalid archive size or members' }
  $bundle = Join-Path $tmp 'bundle'; New-Item -ItemType Directory $bundle | Out-Null
  foreach ($entry in $zip.Entries) { [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, (Join-Path $bundle $entry.FullName)) }
 } finally { $zip.Dispose() }
 if ([IO.File]::ReadAllText((Join-Path $bundle 'RELEASE_VERSION')) -cne "$Version`n") { throw 'Bundle release version mismatch' }
 # A child process prevents the offline installer's exit from skipping our cleanup.
 & (Get-Process -Id $PID).Path -NoProfile -File "$bundle\install.ps1" -Bundle $bundle -Mode $Mode
 if ($LASTEXITCODE -ne 0) { throw 'Installer failed' }
} finally { Remove-Item -Recurse -Force $tmp }
