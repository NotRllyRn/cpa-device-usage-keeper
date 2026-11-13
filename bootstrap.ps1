param([Parameter(Mandatory=$true)][ValidatePattern('\Av[0-9]+\.[0-9]+\.[0-9]+\z')][string]$Version,
 [ValidateSet('menu','install','repair','sources','status','uninstall')][string]$Mode = 'menu')
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Windows required' }
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($arch -eq 'x64') { $arch = 'amd64' }
if ($arch -notin @('amd64','arm64')) { throw 'Supported: Windows x64/arm64' }
if ([Console]::IsInputRedirected) { throw 'Run the downloaded script in an interactive PowerShell terminal' }
$asset = "device-usage-$Version-windows-$arch.zip"
$base = "https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/download/$Version"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory $tmp | Out-Null
function Fetch([string]$Name, [string]$Path, [long]$Limit) {
 & curl.exe --fail --silent --show-error --location --max-redirs 5 --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 20 --max-time 180 --max-filesize $Limit "$base/$Name" -o $Path
 if ($LASTEXITCODE -ne 0) { throw 'Release download failed' }
 if ((Get-Item $Path).Length -gt $Limit) { throw 'Download too large' }
}
try {
 Fetch "$asset.sha256" "$tmp\checksum" 1024
 Fetch $asset "$tmp\archive.zip" 134217728
 $checksum = [IO.File]::ReadAllText("$tmp\checksum")
 if ($checksum -cnotmatch ('\A([a-f0-9]{64})  ' + [regex]::Escape($asset) + '\n\z')) { throw 'Invalid checksum manifest' }
 if ((Get-FileHash "$tmp\archive.zip" -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Matches[1]) { throw 'Release checksum mismatch' }
 Add-Type -AssemblyName System.IO.Compression.FileSystem
 $zip = [IO.Compression.ZipFile]::OpenRead("$tmp\archive.zip")
 try {
  $expected = @('install.sh','install-macos.sh','install.ps1','LICENSE','BUILD.txt','SHA256SUMS','collector.tgz',"device-sync-windows-$arch.exe")
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
 # A child process prevents the offline installer's exit from skipping our cleanup.
 & (Get-Process -Id $PID).Path -NoProfile -File "$bundle\install.ps1" -Bundle $bundle -Mode $Mode
 if ($LASTEXITCODE -ne 0) { throw 'Installer failed' }
} finally { Remove-Item -Recurse -Force $tmp }
