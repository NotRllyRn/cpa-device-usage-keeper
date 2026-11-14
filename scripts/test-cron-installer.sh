#!/usr/bin/env bash
# Synthetic binaries and private mocked crontab only; never installs on the host.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home space % ' \$ \\ end" PATH="$tmp/bin:$PATH"
export CRON_TABLE="$tmp/table" CRON_DAEMON=1 SYSTEMD_READY=0
mkdir -p "$HOME" "$tmp/bin" "$tmp/bundle/package/bin"
cat > "$tmp/bin/systemctl" <<'EOF'
#!/bin/sh
exit "${SYSTEMD_READY:-1}"
EOF
# 1 means unavailable for systemctl, unlike the daemon switch.
export SYSTEMD_READY=1
cat > "$tmp/bin/pgrep" <<'EOF'
#!/bin/sh
[ "$CRON_DAEMON" = 1 ]
EOF
cat > "$tmp/bin/crontab" <<'EOF'
#!/bin/sh
if [ "$1" = -l ]; then
  if [ "${FAIL_READ:-0}" = 1 ]; then echo 'Permission denied' >&2; exit 1; fi
  if [ -f "$CRON_TABLE" ]; then cat "$CRON_TABLE"; else echo "${NO_TABLE_MESSAGE:-no crontab for test}" >&2; exit 1; fi
elif [ "${FAIL_CRON:-0}" = 1 ] && grep -q 'managed schedule' "$1"; then
  exit 1
else cp "$1" "$CRON_TABLE"; fi
EOF
cat > "$tmp/bin/sha512sum" <<'EOF'
#!/bin/sh
cat >/dev/null
EOF
cat > "$tmp/bundle/device-sync-linux-amd64" <<'EOF'
#!/bin/sh
case "$1" in
configure)
 mkdir -p "$HOME/.local/state/device-usage-keeper"
 printf '{}\n' > "$HOME/.local/state/device-usage-keeper/config.json"
 printf '{}\n' > "$HOME/.local/state/device-usage-keeper/state.json" ;;
sync)
 printf '%s\n%s\n%s\n' "$HOME" "$PWD" "$PATH" > "$HOME/environment"
 echo 'SECRET must never be logged' >&2
 exit 1 ;;
esac
EOF
cat > "$tmp/bundle/package/bin/ccusage" <<'EOF'
#!/bin/sh
printf 'ccusage 20.0.43\n'
EOF
chmod +x "$tmp/bin/"* "$tmp/bundle/device-sync-linux-amd64" "$tmp/bundle/package/bin/ccusage"
export DEVICE_USAGE_BUNDLE="$tmp/bundle"
refresh() { (cd "$tmp/bundle" && sha256sum device-sync-linux-amd64 collector.tgz RELEASE_VERSION > SHA256SUMS); }
printf 'v0.1.0\n' > "$tmp/bundle/RELEASE_VERSION"
(cd "$tmp/bundle" && tar czf collector.tgz package)
refresh
printf '# unrelated  %% entry\nMAILTO="example@example.invalid"\n0 1 * * * echo untouched\n\n' > "$CRON_TABLE"
cp "$CRON_TABLE" "$tmp/original"
# Simulate a missing command without depending on host packages.
command() {
  if [[ "$1" == -v && "$2" == "${MISSING_TOOL:-}" ]]; then return 1; fi
  builtin command "$@"
}
export -f command
if MISSING_TOOL=crontab bash "$root/install.sh" install >"$tmp/error" 2>&1; then exit 1; fi
grep -q 'requires crontab' "$tmp/error"
if CRON_DAEMON=0 bash "$root/install.sh" install >"$tmp/error" 2>&1; then exit 1; fi
grep -q 'running cron/crond' "$tmp/error"
test ! -e "$HOME/.local/bin/device-sync"
# A failed configure/scheduling install must remove binaries and leave private state.
if FAIL_CRON=1 bash "$root/install.sh" install >"$tmp/error" 2>&1; then exit 1; fi
cmp "$CRON_TABLE" "$tmp/original"
test ! -e "$HOME/.local/bin/device-sync"
test -f "$HOME/.local/state/device-usage-keeper/config.json"
# Fresh test home (do not erase retained real installation state).
export HOME="$tmp/fresh space % ' \$ \\ end"; mkdir -p "$HOME"
MISSING_TOOL=systemctl bash "$root/install.sh" install >/dev/null
test "$(<"$HOME/.local/lib/device-usage-keeper/scheduler")" = cron
if bash "$root/install.sh" install >/dev/null 2>&1; then exit 1; fi
cp "$CRON_TABLE" "$tmp/installed"
# Run the exact cron command under a minimal environment. Encoded path has no %.
line="$(tail -n 1 "$CRON_TABLE")"; command="${line#* * * * * }"
env -i HOME=/wrong PATH=/usr/bin:/bin /bin/sh -c "$command"
test "$(head -n 1 "$HOME/environment")" = "$HOME"
test "$(head -n 2 "$HOME/environment" | tail -n 1)" = "$HOME"
test "$(tail -n 1 "$HOME/environment")" = /usr/local/bin:/usr/bin:/bin
test "$(<"$HOME/.local/lib/device-usage-keeper/cron-status")" = 'Sync failed; run device-sync status'
test "$(stat -c %a "$HOME/.local/lib/device-usage-keeper/cron-status")" = 600
before="$(sha256sum "$HOME/.local/bin/device-sync")"
config_before="$(sha256sum "$HOME/.local/state/device-usage-keeper/config.json")"
printf '# new release\n' >> "$tmp/bundle/device-sync-linux-amd64"
printf 'v0.1.1\n' > "$tmp/bundle/RELEASE_VERSION"; refresh
# One-shot schedule failure allows rollback, persistent failure reports recovery files.
cp "$tmp/bin/crontab" "$tmp/crontab-backup"
python3 - "$tmp/bin/crontab" <<'PY'
import sys
p=sys.argv[1]
s=open(p).read().replace('exit 1\nelse cp', 'if [ ! -f "$CRON_TABLE.failed" ]; then touch "$CRON_TABLE.failed"; exit 1; fi\n  cp "$1" "$CRON_TABLE"\nelse cp')
open(p,'w').write(s)
PY
if FAIL_CRON=1 bash "$root/install.sh" update >"$tmp/error" 2>&1; then exit 1; fi
cmp "$CRON_TABLE" "$tmp/installed"
test "$(<"$HOME/.local/lib/device-usage-keeper/release-version")" = v0.1.0
test "$(sha256sum "$HOME/.local/bin/device-sync")" = "$before"
test "$(sha256sum "$HOME/.local/state/device-usage-keeper/config.json")" = "$config_before"
cp "$tmp/crontab-backup" "$tmp/bin/crontab"
if FAIL_CRON=1 bash "$root/install.sh" update >"$tmp/error" 2>&1; then exit 1; fi
grep -q 'Rollback incomplete' "$tmp/error"
recovery="$(awk '/Rollback incomplete/ {print $NF}' "$tmp/error")"
test -f "$recovery/old-cron"; cp "$recovery/old-cron" "$CRON_TABLE"; rm -rf "$recovery"
# Even when systemd becomes usable, update must keep cron.
SYSTEMD_READY=0 bash "$root/install.sh" update >/dev/null
bash "$root/install.sh" update >/dev/null
cmp "$CRON_TABLE" "$tmp/installed"
bash "$root/install.sh" status >/dev/null
# Uninstall works without a running daemon.
CRON_DAEMON=0 bash "$root/install.sh" uninstall >/dev/null
cmp "$CRON_TABLE" "$tmp/original"
test -f "$HOME/.local/state/device-usage-keeper/state.json"
test ! -e "$HOME/.local/lib/device-usage-keeper/scheduler"
export HOME="$tmp/no-table"; mkdir -p "$HOME"; rm "$CRON_TABLE"
if FAIL_READ=1 bash "$root/install.sh" install >"$tmp/error" 2>&1; then exit 1; fi
grep -q 'Cannot read user crontab' "$tmp/error"
test ! -e "$HOME/.local/bin/device-sync"
NO_TABLE_MESSAGE="crontab: can't open 'test': No such file or directory" bash "$root/install.sh" install >/dev/null
bash "$root/install.sh" uninstall >/dev/null
test ! -s "$CRON_TABLE"
export HOME="$tmp/no-table-vixie"; mkdir -p "$HOME"; rm "$CRON_TABLE"
bash "$root/install.sh" install >/dev/null
bash "$root/install.sh" uninstall >/dev/null
test ! -s "$CRON_TABLE"
echo 'PASS: cron prerequisites, rollback, quoting/minimal environment, private bounded output, persisted scheduler, no duplicates, unrelated entries and state retained'
