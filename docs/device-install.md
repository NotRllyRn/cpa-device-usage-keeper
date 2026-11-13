# Device collector and installation

For macOS and Windows private test bundles, see [device platforms](device-platforms.md).
All installers now offer [source discovery and selection](device-sources.md): Enter
accepts all checked roots; numbered toggles deselect them. Saved consent is enforced
on scheduled runs, not just displayed by the installer.
The Linux release workflow below is separate from those platform installers.

The uploader is implemented locally. Private test bundles can now be built and
installed without a public release; follow [CPA and device testing](cpa-testing.md).
**No approved public release binaries exist yet**, so the hosted release command
below is not currently a working public installation offer. Do not
run it on this machine or production as part of development.

## Release installation contract

Initial support: Linux x86_64 with a working user systemd instance. No root,
new npm package, or end-user compiler is required. Other platforms are unsupported.
The installer downloads a versioned uploader binary and the official pinned
ccusage 20.0.43 Linux binary, verifies SHA256/SHA512, and prompts on `/dev/tty`
for an HTTPS endpoint and hidden upload token. Obtain the upload token through
admin enrollment; never use CPA credentials. Secrets do not go in command lines.

Once a release is approved and published, the intended one-command form is:

```sh
curl --fail --proto '=https' --tlsv1.2 -sS https://raw.githubusercontent.com/NotRllyRn/cpa-device-usage-keeper/APPROVED_TAG/install.sh | DEVICE_USAGE_VERSION=APPROVED_TAG DEVICE_USAGE_SHA256=TRUSTED_BINARY_SHA256 bash
```

Replace the placeholders with a published immutable tag and a trusted checksum.
Prefer downloading and inspecting the script before executing it. Never select
a checksum from an untrusted channel merely to make verification pass.

`bash install.sh update`, `bash install.sh status`, and `bash install.sh uninstall`
manage the same installation. Updates retain private state and restore the prior
binaries/units on ordinary command failure. Power loss/SIGKILL recovery is not
claimed. Uninstall removes scheduling and executables but keeps state; revoke
the device in the dashboard. Re-running install over existing state is rejected.

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
