# Device collector and installation

For macOS and Windows private test bundles, see [device platforms](device-platforms.md).
All installers now offer [source discovery and selection](device-sources.md): Enter
accepts all checked roots; numbered toggles deselect them. Saved consent is enforced
on scheduled runs, not just displayed by the installer.
Hosted bootstraps hand off to these same offline bundle installers.

The uploader is implemented locally. Private test bundles can now be built and
installed without a public release; follow [CPA and device testing](cpa-testing.md).
**No approved public release binaries exist yet**, so the hosted release command
below is not currently a working public installation offer. Do not
run it on this machine or production as part of development.

## Release installation contract

Supported: Linux x86_64 with user systemd, macOS Intel/Apple Silicon, and
Windows x64/ARM64. Linux ARM is not supported. No root, npm or compiler is needed.
Unix requires curl, Python 3 (safe ZIP validation), and Bash; Windows requires
PowerShell and curl.exe. An interactive terminal is required. Obtain an upload
token through admin enrollment; never use CPA credentials or put secrets in commands.

**Unavailable until an approved release with bootstrap assets is published.**
The bootstrap resolves GitHub's latest stable release once and pins downloads to
that validated tag. Drafts, prereleases and development tags are rejected.
Prefer downloading and inspecting scripts.

```sh
curl -fsSL --proto '=https' --proto-redir '=https' --max-time 60 https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/latest/download/bootstrap.sh | sh
```

Windows (download, inspect, then run from an interactive PowerShell terminal):

```powershell
curl.exe -fsSL --proto '=https' --proto-redir '=https' --max-time 60 https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/latest/download/bootstrap.ps1 -o bootstrap.ps1
# Inspect bootstrap.ps1 before running:
.\bootstrap.ps1
```

The bootstrap downloads `device-usage-vX.Y.Z-OS-ARCH.zip` and its `.zip.sha256`
from the same GitHub release, checks SHA256 and rejects unexpected, duplicate,
non-regular or oversized archive entries before invoking the offline installer.
Checksums detect corruption, **not independent authenticity**: these are not signatures.
HTTPS redirects are restricted to HTTPS and downloads have size/time limits.
Unix prompts use the terminal even when the bootstrap arrives through a pipe.
An optional `DEVICE_USAGE_VERSION=vX.Y.Z` override (or PowerShell `-Version vX.Y.Z`)
selects an explicit stable tag instead of querying latest.

Existing installations show installed and available versions before any install
changes. Different or unknown versions require explicit confirmation; declining
leaves the installation unchanged. Older target versions are labeled a downgrade
or version switch, never silently installed. Same-version runs show the existing
action menu, without automatic reinstallation. Windows reports that in-place
upgrade/switch is unsupported and makes no changes even if confirmed.

Successful bundle installs record `~/.local/lib/device-usage-keeper/release-version`
from the checksummed bundle's `RELEASE_VERSION`. Updates record it only after
successful scheduling; ordinary update rollback retains the previous marker.
Legacy installs without a marker are reported as unknown, not guessed from the
CLI's development version string. Private config and upload state are not modified
by version checks.

`bash install.sh update`, `bash install.sh status`, and `bash install.sh uninstall`
manage the same installation. Updates retain private state and restore the prior
binaries/units on ordinary command failure. Power loss/SIGKILL recovery is not
claimed. Uninstall removes scheduling and executables but keeps state; revoke
the device in the dashboard. No-argument runs offer an action menu and detect
partial installs (binaries, scheduler or state), not only config.json. Explicit
`install` over retained state is rejected. Unix update requires a complete install;
partial-state repair is not automated: restore a private backup rather than reset
state. Windows offers repair only with valid retained state and no registered task;
it does not offer in-place update or rollback. See [platform limits](device-platforms.md).

The timer runs every 15 minutes with jitter, while the user systemd instance is
running. The installer does not enable lingering. Custom `DEVICE_USAGE_STATE`
is supported by the CLI for manual use, but deliberately rejected by the
installer so the scheduled service cannot silently use a different directory.

## Local state and collection

- Executable: `~/.local/bin/device-sync`
- Pinned collector: `~/.local/lib/device-usage-keeper/ccusage`
- Private state: `~/.local/state/device-usage-keeper/`
- Commands: `device-sync configure|sources|drain|sync|status|version`

Configuration initializes state before enabling collection. The state binds to
the HTTPS URL and upload token. Changing either is rejected rather than silently
skipping acknowledged history or moving pending reports. Back up config and state
together with private permissions. If state is lost, restore the backup. Otherwise
enroll a new device, retain/revoke the old record, and configure a fresh directory;
old and replacement devices overlap, so do not sum both. Migration/reconciliation
of server backups or a new device is not automated.

Each sync validates saved source consent, flushes persisted pending payloads, then asks ccusage for approved UTC
daily history using `--by-agent --json --offline`. Only changed snapshots are
queued. The outbox uses fsync, atomic rename, and directory sync. Upload errors
retain it; retryable failures use bounded retries and the next timer run retries
again. Overlapping runs are excluded with a file lock. Redirects are refused.

Source roots are saved in private `config.json`. Use `device-sync sources` or
`install.sh sources` to change them; do not edit `ccusage.json` to control collection.
Updates retain consent, while legacy clients without an allowlist require explicit
approval before syncing. Pending uploads block source changes; `device-sync drain`
explicitly sends only those queued payloads. Deselecting does not delete uploaded
history. See [source selection](device-sources.md) for discovery, supported roots,
Goose/named-store limitations, and the isolation contract. No custom usage parsers
are included. Output is limited
to 32 MiB per collector invocation and total collection to five minutes; larger histories fail visibly rather
than being silently truncated. Historical days missing from a later collection
are retained; explicit smaller reports replace prior totals.

Uploads allowlist day, agent/model identifiers, token aggregates, revision and
collection time. Raw paths, prompts, logs, collector costs and errors are not
uploaded. Identifiers are preserved and may contain custom identifying text;
review your model naming rather than treating allowlisting as anonymization.
Session reports and their path-safe identifiers are not implemented yet.
