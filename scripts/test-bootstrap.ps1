# Mocked Windows bootstrap: no network, scheduler, ACL or real installation.
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
$oldOS = $env:OS
New-Item -ItemType Directory $tmp | Out-Null
function Assert($condition, $message) { if (!$condition) { throw $message } }
function Get-ScheduledTask { return $null }
function Read-Host { return $script:answer }
function curl.exe {
 $url = $args | Where-Object { $_ -like 'https://*' }
 $outIndex = [array]::IndexOf($args, '-o')
 $dest = $args[$outIndex + 1]
 $script:downloads += $url
 if ($url.EndsWith('/releases/latest')) { [IO.File]::WriteAllText($dest, $script:response) }
 elseif ($url.EndsWith('.sha256')) { Copy-Item "$tmp\checksum" $dest }
 elseif ($url.EndsWith('.zip')) { Copy-Item "$tmp\archive.zip" $dest }
 else { throw 'Unexpected URL' }
 $global:LASTEXITCODE = 0
}
try {
 $env:OS = 'Windows_NT'; $env:TEST_HOME = Join-Path $tmp 'home'
 $env:TEST_INVOKED = Join-Path $tmp 'invoked'
 $script:answer = 'n'
 # Test-only substitutions isolate HOME and permit execution in a CI pipe.
 $source = [IO.File]::ReadAllText((Join-Path $root 'bootstrap.ps1')).Replace('$HOME', '$env:TEST_HOME').Replace('[Console]::IsInputRedirected', '$false')
 $bootstrap = Join-Path $tmp 'bootstrap.ps1'
 [IO.File]::WriteAllText($bootstrap, $source)
 $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
 if ($arch -eq 'x64') { $arch = 'amd64' }
 $asset = "device-usage-v1.2.3-windows-$arch.zip"
 $bundle = Join-Path $tmp 'bundle'; New-Item -ItemType Directory $bundle | Out-Null
 foreach ($name in @('install.sh','install-macos.sh','LICENSE','BUILD.txt','SHA256SUMS','collector.tgz',"device-sync-windows-$arch.exe")) { [IO.File]::WriteAllText((Join-Path $bundle $name), 'fixture') }
 [IO.File]::WriteAllText("$bundle\RELEASE_VERSION", "v1.2.3`n")
 [IO.File]::WriteAllText("$bundle\install.ps1", 'param($Bundle,$Mode); [IO.File]::WriteAllText($env:TEST_INVOKED, $Mode)')
 Add-Type -AssemblyName System.IO.Compression.FileSystem
 [IO.Compression.ZipFile]::CreateFromDirectory($bundle, "$tmp\archive.zip")
 [IO.File]::WriteAllText("$tmp\checksum", (Get-FileHash "$tmp\archive.zip" -Algorithm SHA256).Hash.ToLowerInvariant()+"  $asset`n")
 foreach ($bad in @('{}','[]','invalid','{"tag_name":"dev","draft":false,"prerelease":false}','{"tag_name":"v1.2.3","draft":false,"prerelease":true}')) {
  $script:response = $bad; $failed = $false
  try { & $bootstrap -Version '' } catch { $failed = $true }
  Assert $failed 'Bad latest response was accepted'
 }
 $script:response = '{"tag_name":"v1.2.3","draft":false,"prerelease":false}'
 $script:downloads = @(); & $bootstrap -Version ''
 Assert ($script:downloads.Count -eq 3) 'Latest was not resolved exactly once'
 Assert ([IO.File]::ReadAllText($env:TEST_INVOKED) -eq 'menu') 'Fresh installation did not reach menu'
 Remove-Item $env:TEST_INVOKED
 $lib = Join-Path $env:TEST_HOME '.local\lib\device-usage-keeper'
 New-Item -ItemType Directory -Force $lib | Out-Null
 foreach ($saved in @('unknown', 'v1.0.0', 'v2.0.0')) {
  [IO.File]::WriteAllText("$lib\release-version", "$saved`n")
  $script:downloads = @(); & $bootstrap -Version ''
  Assert ($script:downloads.Count -eq 1) 'Decline downloaded an archive'
  Assert (!(Test-Path $env:TEST_INVOKED)) 'Decline invoked installer'
  Assert ([IO.File]::ReadAllText("$lib\release-version") -eq "$saved`n") 'Decline changed installed version'
 }
 $script:answer = 'yes'; $failed = $false
 try { & $bootstrap -Version '' } catch { $failed = $_.Exception.Message -like 'Upgrade blocked:*' }
 Assert $failed 'Windows upgrade was not blocked honestly'
 [IO.File]::WriteAllText("$lib\release-version", "v1.2.3`n")
 & $bootstrap -Version ''
 Assert ([IO.File]::ReadAllText($env:TEST_INVOKED) -eq 'menu') 'Same version did not reach menu'
 Write-Output 'PASS: Windows latest parsing, fresh/same menu, different/legacy decline, unsupported upgrade guard (mocked)'
} finally {
 $env:OS = $oldOS
 Remove-Item Env:TEST_HOME,Env:TEST_INVOKED -ErrorAction SilentlyContinue
 Remove-Item -Recurse -Force $tmp
}
