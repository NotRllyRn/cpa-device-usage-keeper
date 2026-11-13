param([ValidateSet('install','repair','update','sources','status','uninstall')][string]$Mode = 'install', [string]$Bundle = $env:DEVICE_USAGE_BUNDLE)
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Windows required' }
if ($env:DEVICE_USAGE_STATE) { throw 'Custom state requires manual scheduling' }
$bin = Join-Path $HOME '.local\bin'
$lib = Join-Path $HOME '.local\lib\device-usage-keeper'
$state = Join-Path $HOME '.local\state\device-usage-keeper'
$exe = Join-Path $bin 'device-sync.exe'
$task = 'DeviceUsageKeeper'
if ($Mode -eq 'sources') { & $exe sources; exit $LASTEXITCODE }
if ($Mode -eq 'status') { Get-ScheduledTask -TaskName $task; & $exe status; exit $LASTEXITCODE }
if ($Mode -eq 'uninstall') {
 Stop-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue
 Unregister-ScheduledTask -TaskName $task -Confirm:$false -ErrorAction SilentlyContinue
 Remove-Item $exe,(Join-Path $lib 'ccusage.exe') -ErrorAction SilentlyContinue
 Write-Output 'Uninstalled; private state retained. Revoke the device in the dashboard.'; exit
}
if (!$Bundle) { throw 'Supply -Bundle with a trusted extracted Windows bundle directory' }
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($arch -eq 'x64') { $arch = 'amd64' }
if ($arch -notin @('amd64','arm64')) { throw 'Unsupported Windows architecture' }
foreach ($line in Get-Content (Join-Path $Bundle 'SHA256SUMS')) {
 if ($line -notmatch '^([a-fA-F0-9]{64})  ([a-zA-Z0-9._-]+)$') { throw 'Invalid checksum manifest' }
 if ((Get-FileHash (Join-Path $Bundle $Matches[2]) -Algorithm SHA256).Hash -ne $Matches[1]) { throw 'Bundle checksum mismatch' }
}
if ($Mode -eq 'install' -and ((Test-Path "$state\config.json") -or (Test-Path "$state\state.json"))) { throw 'Already configured; use update' }
if ($Mode -eq 'update') { throw 'Windows in-place update is not yet validated. Preserve state and use a reviewed manual binary replacement with the scheduled task stopped.' }
New-Item -ItemType Directory -Force $bin,$lib,$state | Out-Null
# Protected ACL: current account and SYSTEM only, inherited by state files.
$sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
$acl = New-Object System.Security.AccessControl.DirectorySecurity
$acl.SetOwner($sid); $acl.SetAccessRuleProtection($true,$false)
foreach ($identity in @($sid, [System.Security.Principal.SecurityIdentifier]'S-1-5-18')) {
 $rule = New-Object System.Security.AccessControl.FileSystemAccessRule($identity,'FullControl','ContainerInherit,ObjectInherit','None','Allow')
 $acl.AddAccessRule($rule)
}
Set-Acl -Path $state -AclObject $acl
$tmp = Join-Path $env:TEMP ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory $tmp | Out-Null
try {
 & tar.exe -xzf (Join-Path $Bundle 'collector.tgz') -C $tmp 'package/bin/ccusage.exe'
 if ($LASTEXITCODE -ne 0) { throw 'Collector extraction failed' }
 Copy-Item (Join-Path $Bundle "device-sync-windows-$arch.exe") $exe
 Copy-Item "$tmp\package\bin\ccusage.exe" "$lib\ccusage.exe"
 if ((& "$lib\ccusage.exe" --version) -ne 'ccusage 20.0.43') { throw 'Collector version mismatch' }
 if ($Mode -eq 'repair') {
  & $exe status
  if ($LASTEXITCODE -ne 0) { throw 'Cannot repair without valid retained state' }
 } else {
  & $exe configure
  if ($LASTEXITCODE -ne 0) { throw 'Configuration failed; scheduler not registered. Use -Mode repair after resolving the error if configuration was saved.' }
 }
 $action = New-ScheduledTaskAction -Execute $exe -Argument 'sync'
 $trigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(2) -RepetitionInterval (New-TimeSpan -Minutes 15)
 $principal = New-ScheduledTaskPrincipal -UserId $sid.Value -LogonType Interactive -RunLevel Limited
 $settings = New-ScheduledTaskSettingsSet -MultipleInstances IgnoreNew -ExecutionTimeLimit (New-TimeSpan -Minutes 15) -StartWhenAvailable
 Register-ScheduledTask -Force -TaskName $task -Action $action -Trigger $trigger -Principal $principal -Settings $settings | Out-Null
 Write-Output 'Installed for the current logged-in user, every 15 minutes. Run ~/.local/bin/device-sync.exe status.'
} finally { Remove-Item -Recurse -Force $tmp }
