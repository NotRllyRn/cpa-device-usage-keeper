# Device platforms

The backend plugin remains Linux/amd64. Device clients now cross-build independently
of SQLite and server code for Linux/amd64, macOS arm64/amd64, and Windows arm64/amd64.
They use official pinned ccusage 20.0.43 platform binaries. No Node/npm installation
is needed on devices. These are private test candidates, not signed public releases.

Build all client bundles: `python3 scripts/build-device-bundles.py /new/output/path`.
Each subdirectory has a platform binary, verified collector archive, installers,
and SHA256SUMS. Transfer the matching directory via a trusted channel.
Initial setup uses the same [source selection menu](device-sources.md) on every
platform. Enter accepts all checked roots; type numbers to deselect or add manual
paths. Scheduled sync never discovers new roots. Updates/repair preserve consent;
legacy configurations without approved sources stop collecting until reconfigured. A checksum
manifest alongside the files detects corruption, not a malicious bundle replacement.

## macOS

Select `darwin-arm64` for Apple Silicon or `darwin-amd64` for Intel. With the bundle
in a private local directory:

```sh
DEVICE_USAGE_BUNDLE=/path/to/bundle bash /path/to/bundle/install.sh
```

`install.sh` dispatches to `install-macos.sh`. Enter the HTTPS server and a newly
enrolled device's upload token. It creates a per-user LaunchAgent, runs at login
and every 15 minutes while logged in, and keeps state in
`~/.local/state/device-usage-keeper`. Use `~/.local/bin/device-sync sync` or `status`.
Use `install.sh update|sources|status|uninstall` with the same bundle variable. Uninstall
retains state and history. Installer updates attempt rollback on ordinary command
failure; power loss recovery is not claimed. Unsigned binaries may require local
security approval; do not disable Gatekeeper globally.

## Windows

Use `windows-amd64` for x64 or `windows-arm64` for ARM64, in PowerShell:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:\path\bundle\install.ps1 -Bundle C:\path\bundle
```

Inspect the script first. The execution-policy override applies to this process,
not the machine. Existing organizational policy may still prevent execution.
The installer sets private state ACLs and prompts for the endpoint/token without
putting credentials in command arguments. A current-user, non-elevated scheduled
task runs every 15 minutes while logged in. The client also enforces private ACLs
for manual configuration and custom state directories.

```powershell
& "$HOME\.local\bin\device-sync.exe" sync
& "$HOME\.local\bin\device-sync.exe" status
```

Use `-Mode sources`, `-Mode status` or `-Mode uninstall`. If task registration failed after successful
configuration, use `-Mode repair -Bundle C:\path\bundle` to restore scheduling
without resetting history revisions. Windows in-place update is deliberately
rejected until its rollback path is implemented and tested.

## Validation boundary

Linux tests, race tests and cross-compilation pass. macOS/Windows native execution,
terminal prompts, launchd and Task Scheduler remain unverified on their native OS.
Do not mistake cross-compilation for native certification. Keep configuration and
state backups together; do not delete state to work around installer errors.
The global bootstrap currently dispatches Linux/macOS Bash and a separate Windows
PowerShell entry point. Hosted OS detection/download bootstrap is not published.
