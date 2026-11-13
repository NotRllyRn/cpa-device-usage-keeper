param([ValidateSet('menu','cancel','install','repair','update','sources','status','uninstall')][string]$Mode = 'menu', [string]$Bundle = $env:DEVICE_USAGE_BUNDLE)
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Windows required' }
if ($env:DEVICE_USAGE_STATE) { throw 'Custom state requires manual scheduling' }
$bin = Join-Path $HOME '.local\bin'
$lib = Join-Path $HOME '.local\lib\device-usage-keeper'
$state = Join-Path $HOME '.local\state\device-usage-keeper'
$exe = Join-Path $bin 'device-sync.exe'
$task = 'DeviceUsageKeeper'
if ($Mode -eq 'menu') {
 if ((Test-Path $state) -or (Test-Path $exe) -or (Test-Path "$lib\ccusage.exe") -or (Get-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue)) {
  Write-Output 'Existing or partial installation found. In-place update is not supported. Repair is only for retained valid state without a registered task.'
 }
 $Mode = Read-Host 'Choose: install / repair / sources / status / uninstall / cancel'
 if ($Mode -notin @('install','repair','sources','status','uninstall','cancel','')) { throw 'Invalid selection' }
}
if ($Mode -in @('cancel','')) { exit }
if ($Mode -eq 'sources') { & $exe sources; exit $LASTEXITCODE }
if ($Mode -eq 'status') { Get-ScheduledTask -TaskName $task; & $exe status; exit $LASTEXITCODE }
if ($Mode -eq 'uninstall') {
 $existing = Get-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue
 if ($existing) {
  Stop-ScheduledTask -TaskName $task
  for ($attempt = 0; $attempt -lt 30; $attempt++) {
   if ((Get-ScheduledTask -TaskName $task).State -ne 'Running') { break }
   Start-Sleep -Seconds 1
  }
  if ((Get-ScheduledTask -TaskName $task).State -eq 'Running') { throw 'Task did not stop; files retained' }
  Unregister-ScheduledTask -TaskName $task -Confirm:$false
 }
 Remove-Item $exe,(Join-Path $lib 'ccusage.exe'),(Join-Path $lib 'release-version') -ErrorAction SilentlyContinue
 Write-Output 'Uninstalled; private state retained. Revoke the device in the dashboard.'; exit
}
if ($Mode -in @('install','repair') -and (Get-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue)) { throw 'A task already exists. In-place replacement is unsupported; uninstall first (private state is retained), then repair.' }
if ($Mode -eq 'repair' -and (!(Test-Path "$state\config.json") -or !(Test-Path "$state\state.json"))) { throw 'Repair requires retained config and state; restore a backup first' }
if (!$Bundle) { throw 'Supply -Bundle with a trusted extracted Windows bundle directory' }
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($arch -eq 'x64') { $arch = 'amd64' }
if ($arch -notin @('amd64','arm64')) { throw 'Unsupported Windows architecture' }
foreach ($line in Get-Content (Join-Path $Bundle 'SHA256SUMS')) {
 if ($line -notmatch '^([a-fA-F0-9]{64})  ([a-zA-Z0-9._-]+)$') { throw 'Invalid checksum manifest' }
 if ((Get-FileHash (Join-Path $Bundle $Matches[2]) -Algorithm SHA256).Hash -ne $Matches[1]) { throw 'Bundle checksum mismatch' }
}
if ($Mode -eq 'install' -and ((Test-Path "$state\config.json") -or (Test-Path "$state\state.json"))) { throw 'Already configured; use repair after uninstalling the scheduler (state is retained)' }
if ($Mode -eq 'update') { throw 'Windows in-place update is not yet validated. Preserve state and use a reviewed manual binary replacement with the scheduled task stopped.' }
$releaseVersion = 'unknown'
if (Test-Path (Join-Path $Bundle 'RELEASE_VERSION')) {
 $releaseVersion = [IO.File]::ReadAllText((Join-Path $Bundle 'RELEASE_VERSION')).TrimEnd("`n")
 if ($releaseVersion -cnotmatch '\Av[0-9]+\.[0-9]+\.[0-9]+\z') { throw 'Invalid bundle release version' }
}
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
 [IO.File]::WriteAllText((Join-Path $lib 'release-version.new'), "$releaseVersion`n")
 Move-Item -Force (Join-Path $lib 'release-version.new') (Join-Path $lib 'release-version')
 Write-Output 'Installed for the current logged-in user, every 15 minutes. Run ~/.local/bin/device-sync.exe status.'
} finally { Remove-Item -Recurse -Force $tmp }
