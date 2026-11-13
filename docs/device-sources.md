# Choose device usage sources

All three installers use the same Go terminal menu. Initial setup and the explicit
`sources` command discover local usage roots and start with every selectable root
checked. Enter accepts the checked list, shows the selected paths, then Enter
confirms saving. Numbers toggle entries; `add <agent> <absolute path>` adds a manual
root (paths may contain spaces). EOF/cancel does not save. No new terminal library
is required.

Example (paths and discovered entries vary):

```text
HTTPS server URL: https://usage.example.com
Upload-only token (hidden):
Approve whole usage roots, including future files beneath them. No uploaded history is deleted by deselection.
Manual agents: amp, antigravity, claude, codebuff, codex, copilot, copilot-otel, droid, gemini, goose, grok, hermes, kilo, kimi, openclaw, opencode, pi, qwen, zcode
1. [x] codex  /home/user/.codex
2. [x] pi  /home/user/.config/pi-t3/agent/sessions
3. [x] pi  /home/user/.pi/agent/sessions
Toggle numbers (e.g. 1 3), add <agent> <absolute path>, or Enter to accept all checked: 3
1. [x] codex  /home/user/.codex
2. [x] pi  /home/user/.config/pi-t3/agent/sessions
3. [ ] pi  /home/user/.pi/agent/sessions
Toggle numbers (e.g. 1 3), add <agent> <absolute path>, or Enter to accept all checked:
Selected roots:
  [x] codex  /home/user/.codex
  [x] pi  /home/user/.config/pi-t3/agent/sessions
Save this selection? [Y/n]:
```

A root grants access to that agent's usage under the entire root, including future
files. It is not a per-session filter. Nested roots for the same agent are rejected
rather than double counted; an unchecked child cannot be excluded while its parent
is checked. This also applies to an unchecked `copilot-otel` file beneath a
checked Copilot home's `otel` directory. Select the narrower roots instead. Paths are canonicalized at setup;
missing, unreadable, changed/symlink-retargeted, unknown or empty selections fail
closed. You may confirm an empty selection to disable collection; scheduled runs
then refuse to collect or upload. Commas in paths are unsupported because the
collector uses comma lists.

## Consent and existing installations

Scheduled runs read only `config.json`'s saved `sources` array. They **never run
discovery**. Newly installed agents or new roots do not become approved on the next
sync. New files inside an approved root are included.

Updates/Windows repair preserve configuration and do not display a new all-selected
menu or silently expand consent. This deliberately differs from a new interactive
setup. Run `sources` when you explicitly want a fresh menu; that command starts with
all discovered and previously saved roots checked, and shows them before saving.
Do not accept blindly if you previously excluded some of them.

Older installations have no saved allowlist. After replacing the client they stop
syncing until you explicitly approve sources. Existing `ccusage.json` path settings
can be offered in the menu, but that file no longer controls collection. Do not edit
JSON to bypass the source-change guard.

```sh
# Linux and macOS, using the installed client:
~/.local/bin/device-sync sources
# Equivalent installer entrypoint:
bash /path/to/bundle/install.sh sources
```

```powershell
# Windows:
& "$HOME\.local\bin\device-sync.exe" sources
# Equivalent installer entrypoint:
powershell -NoProfile -ExecutionPolicy Bypass -File C:\path\bundle\install.ps1 -Mode sources
```

Source changes hold the sync lock and are refused while any outbox payload is
pending. Those payloads may contain data you now want to deselect. If you consent
to sending those already queued aggregates, run `device-sync drain`, then `sources`.
`drain` does not collect anything, including on legacy installations. There is no
automatic discard/reset: a failed upload may already have been accepted by the
server, so dropping it without acknowledgment can corrupt revision handling. If
you do not want it uploaded, stop scheduling and retain the pending state; do not
reset it. Source selection does not alter the destination, revisions, digests or
pending payloads.

Deselecting does **not delete uploaded historic data**. Existing behavior remains:
changed day/agent snapshots replace earlier totals with a higher revision; days or
agents absent from later collection remain on the server. No tombstones or history
deletions are added by this feature.

## Discovery

Discovery checks known paths, environment variables in the interactive setup
process, and documented ccusage path settings. It does not scan the disk, inspect
agent credentials or read session contents. Executable presence alone is not proof
of an available usage store. Shell aliases, service environments, arbitrary custom
config files and unexported shell variables cannot be inferred; add those roots
manually. On all platforms, `~` below means the current user's home.

- **Claude:** `~/.claude`, `~/.config/claude`, `$XDG_CONFIG_HOME/claude`, and
  `CLAUDE_CONFIG_DIR` (comma-separated). A config root must contain `projects`;
  selecting `projects` itself normalizes to its parent.
- **Codex:** `~/.codex`, `CODEX_HOME` (comma-separated). Includes both `sessions`
  and `archived_sessions` beneath each approved Codex home.
- **Pi:** `~/.pi/agent/sessions`, `~/.config/pi-t3/agent/sessions`,
  `$XDG_CONFIG_HOME/pi-t3/agent/sessions`, `PI_AGENT_DIR` (ccusage's comma-separated
  session roots), and `$PI_CODING_AGENT_DIR/sessions` (Pi's agent-directory setting).
  Manual Pi roots must be session directories, not the parent agent directory.
- **OpenCode:** `~/.local/share/opencode`, `$XDG_DATA_HOME/opencode`,
  `OPENCODE_DATA_DIR`.
- **Amp:** `~/.local/share/amp`, `AMP_DATA_DIR`.
- **Droid:** `~/.factory/sessions`, `DROID_SESSIONS_DIR`.
- **Codebuff:** `~/.config/manicode`, `manicode-dev`, `manicode-staging`,
  `CODEBUFF_DATA_DIR` (collector recognizes their projects directories).
- **Hermes:** `~/.hermes`, `HERMES_HOME` (roots containing `state.db`).
- **Kilo:** `~/.local/share/kilo`, `KILO_DATA_DIR`.
- **Copilot:** `~/.copilot`, `COPILOT_HOME`; separately selectable
  `COPILOT_OTEL_FILE_EXPORTER_PATH` as `copilot-otel` (one telemetry file).
- **Gemini:** `~/.gemini/tmp`, `GEMINI_DATA_DIR`.
- **Antigravity:** `conversations` beneath `~/.gemini/antigravity`,
  `antigravity-cli`, `antigravity-ide`, `antigravity-backup`, `antigravity-acp`,
  and `~/.config/antigravity`; `ANTIGRAVITY_DATA_DIR`.
- **Kimi:** `~/.kimi`, `~/.kimi-code`, `KIMI_DATA_DIR`.
- **Qwen:** `~/.qwen`, `QWEN_DATA_DIR`.
- **OpenClaw:** `~/.openclaw`, `~/.clawdbot`, `~/.moltbot`, `~/.moldbot`,
  `OPENCLAW_DIR`.
- **Grok:** `~/.grok`, `GROK_HOME`.
- **ZCode:** `~/.zcode`, `ZCODE_HOME`.
- **Goose limitation:** only `GOOSE_PATH_ROOT` or manually supplied roots with
  `data/sessions/sessions.db` can be isolated by pinned ccusage. Its three default
  database locations are detected and reported as **unsupported, not selected**.
  ccusage cannot point at an arbitrary Goose database or exclude individual default
  databases. This client does not copy live databases, create symlinks, or introduce
  another parser to work around that restriction.

All listed override variables except `GOOSE_PATH_ROOT`, `GROK_HOME`, `COPILOT_HOME`
and `COPILOT_OTEL_FILE_EXPORTER_PATH` use ccusage comma lists. For the single-path
adapters, multiple approved roots use separate isolated collector invocations and
their sanitized aggregates are summed. Copies of the same history in different
roots can therefore double-count; approve the original, not its backups.

Only ccusage JSON path fields are imported: `piPath` and `openClawPath` under
`defaults`, daily-command settings and the corresponding agent settings. Locations:
`./.ccusage/ccusage.json`, `~/.config/claude/ccusage.json`,
`~/.claude/ccusage.json`, `$CLAUDE_CONFIG_DIR/ccusage.json`, plus the old client's
configured collector-config file. Files must be regular JSON files under 1 MiB.
Other settings are not executed or passed to the collector. Named `pi.stores`
aliases are reported but not imported; manual `pi` roots use the `pi` agent name,
not a custom alias. Do not migrate a named alias if preserving that identity is
required. Arbitrary `--config` locations require manual roots.

## Enforcement and validation

Upstream source and CLI inspection: ccusage tag **v20.0.43**, unified `daily --help`,
`rust/adapters/*/src/paths.rs`, `ccusage-config/src/config.rs`, and unified source
resolution. The unified CLI has no agent-exclusion option. Supplying only a Pi path
still discovers other agents, so a menu or a partial ccusage config is insufficient.

Each collection instead gets a fresh private empty HOME/XDG directory, an empty
explicit `--config`, and **every adapter's path override** set to either saved roots
or the private empty directory. No arbitrary process environment is inherited.
The empty Claude root contains `projects` so its override does not error/fallback.
The client rejects unexpected agents in returned aggregates as a second check.
This is a discovery boundary for the pinned collector, not an OS sandbox against a
malicious executable. Collector upgrades require re-auditing discovery behavior.

Tests cover persistence, toggles/Enter/manual input, custom Pi environment discovery,
all explicit exclusion variables, empty/missing/unreadable roots, shared locking,
pending guards, and real pinned-collector synthetic histories with conflicting
ambient environment and config. The integration test never reads actual usage or
uploads anything. Linux and mocked macOS/Windows installer entrypoints are tested;
PowerShell mocks run on Linux when `PWSH` points to a portable PowerShell. Native
macOS/Windows terminal, ACL and scheduler behavior still require testing there.
