#!/usr/bin/env bash
# Device installer: Linux x86_64 with user systemd or an existing cron daemon.
set -euo pipefail
umask 077
if [[ "$(uname -s)" == Darwin ]]; then
  base="${DEVICE_USAGE_BUNDLE:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
  exec bash "$base/install-macos.sh" "$@"
fi
mode="${1:-menu}"
[[ -z "${DEVICE_USAGE_STATE:-}" ]] || { echo 'Installer supports only the default state directory; custom state requires manual scheduling.' >&2; exit 1; }
bin="$HOME/.local/bin"
lib="$HOME/.local/lib/device-usage-keeper"
units="$HOME/.config/systemd/user"
state="$HOME/.local/state/device-usage-keeper"
marker='# device-usage-keeper managed schedule'
scheduler=''
if [[ -f "$lib/scheduler" ]]; then
  scheduler="$(<"$lib/scheduler")"
  [[ "$scheduler" == systemd || "$scheduler" == cron ]] || { echo 'Invalid saved scheduler.' >&2; exit 1; }
elif [[ -e "$units/device-usage-keeper.timer" || -e "$units/device-usage-keeper.service" ]]; then
  scheduler=systemd # Pre-scheduler-marker installations.
fi
cron_ready() {
  command -v crontab >/dev/null || { echo 'Cron fallback requires crontab. Install cron using your container/OS setup.' >&2; return 1; }
  command -v pgrep >/dev/null || { echo 'Cron fallback requires pgrep to check the daemon.' >&2; return 1; }
  pgrep -x cron >/dev/null || pgrep -x crond >/dev/null || { echo 'Cron fallback requires a running cron/crond daemon. Start it through your container supervisor; keep it running across restarts. This installer does not install or start services.' >&2; return 1; }
}
read_cron() {
  if LC_ALL=C crontab -l > "$1" 2> "$1.err"; then return; fi
  # Vixie/Cronie and BusyBox report a missing table differently.
  if grep -Eqi "^no crontab for |^crontab: can't open '.*': No such file or directory$" "$1.err"; then : > "$1"; return; fi
  echo 'Cannot read user crontab; no changes made.' >&2; return 1
}
strip_cron() { awk -v marker="$marker" 'length($0) < length(marker) || substr($0, length($0)-length(marker)+1) != marker' "$1" > "$2"; }
# POSIX shell quoting, also used for HOME in the cron runner.
shell_quote() { printf "'%s'" "${1//\'/\'\\\'\'}"; }
if [[ "$mode" == menu ]]; then
  if [[ -d "$state" || -e "$bin/device-sync" || -e "$lib/ccusage" || -e "$lib/scheduler" || -e "$lib/cron-run" || -e "$units/device-usage-keeper.timer" || -e "$units/device-usage-keeper.service" ]]; then
    printf 'Existing or partial installation found. Update requires a complete installation; partial state needs backup recovery.\n'
  fi
  printf 'Choose: install / update / sources / status / uninstall / cancel: ' >/dev/tty
  read -r mode </dev/tty
fi
case "$mode" in
  cancel|'') exit 0 ;;
  sources) exec "$bin/device-sync" sources ;;
  status)
    if [[ "$scheduler" == cron ]]; then
      printf 'Scheduler: cron (every 15 minutes)\n'
      cron_ready
      crontab -l | grep -F -- "$marker"
      [[ ! -f "$lib/cron-status" ]] || head -c 128 "$lib/cron-status"
    elif [[ "$scheduler" == systemd ]]; then systemctl --user status device-usage-keeper.timer --no-pager
    else echo 'No installed scheduler.' >&2; exit 1; fi
    exec "$bin/device-sync" status ;;
  uninstall)
    if [[ "$scheduler" == cron ]]; then
      tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
      read_cron "$tmp/cron"
      strip_cron "$tmp/cron" "$tmp/clean"
      crontab "$tmp/clean"
    elif [[ "$scheduler" == systemd ]]; then
      systemctl --user disable --now device-usage-keeper.timer
      systemctl --user stop device-usage-keeper.service
      rm -f "$units/device-usage-keeper.timer" "$units/device-usage-keeper.service"
      systemctl --user daemon-reload
    fi
    rm -f "$bin/device-sync" "$lib/ccusage" "$lib/release-version" "$lib/scheduler" "$lib/cron-run" "$lib/cron-status"
    printf 'Removed binaries and scheduler. Private state retained for safe revisions and retry. Revoke this device in the dashboard.\n'
    exit 0 ;;
  install|update) ;;
  *) printf 'Usage: install.sh [install|update|sources|status|uninstall]\n' >&2; exit 1 ;;
esac
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { echo 'Only Linux x86_64 is supported.' >&2; exit 1; }
for tool in curl sha256sum sha512sum tar; do command -v "$tool" >/dev/null || { echo "Missing $tool" >&2; exit 1; }; done
if [[ -z "$scheduler" ]]; then
  if command -v systemctl >/dev/null && systemctl --user show-environment >/dev/null 2>&1; then scheduler=systemd
  else scheduler=cron; fi
fi
if [[ "$scheduler" == cron ]]; then cron_ready
else systemctl --user show-environment >/dev/null || { echo 'Saved systemd scheduler is unavailable; restore the user session before updating.' >&2; exit 1; }; fi
[[ "$HOME" != *$'\n'* && "$HOME" != *$'\r'* ]] || { echo 'HOME must not contain newlines.' >&2; exit 1; }
# Direct binary installation requires an explicit immutable release version and
# trusted SHA256. Hosted bootstraps supply a verified complete bundle instead.
if [[ -z "${DEVICE_USAGE_BUNDLE:-}" ]]; then
: "${DEVICE_USAGE_VERSION:?Set DEVICE_USAGE_VERSION to an approved release tag}"
: "${DEVICE_USAGE_SHA256:?Set DEVICE_USAGE_SHA256 to the trusted device-sync-linux-amd64 SHA256}"
[[ "$DEVICE_USAGE_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ && "$DEVICE_USAGE_SHA256" =~ ^[a-fA-F0-9]{64}$ ]] || { echo 'Invalid version/checksum.' >&2; exit 1; }
fi
if [[ "$mode" == install && ( -e "$state/config.json" || -e "$state/state.json" || -e "$bin/device-sync" || -e "$lib/ccusage" || -e "$lib/scheduler" || -e "$lib/cron-run" || -e "$units/device-usage-keeper.service" || -e "$units/device-usage-keeper.timer" ) ]]; then echo 'Existing or partial installation; use update for complete installs, otherwise restore a backup. Do not delete private state.' >&2; exit 1; fi
if [[ "$mode" == update ]]; then
  required=("$state/config.json" "$state/state.json" "$bin/device-sync" "$lib/ccusage")
  if [[ "$scheduler" == systemd ]]; then required+=("$units/device-usage-keeper.service" "$units/device-usage-keeper.timer"); else required+=("$lib/cron-run"); fi
  for file in "${required[@]}"; do
    [[ -f "$file" ]] || { echo 'Incomplete installation; restore from backup before update.' >&2; exit 1; }
  done
fi
tmp="$(mktemp -d)"; updating=false; timer_active=false; timer_enabled=false; cron_changed=false
files=("$bin/device-sync" "$lib/ccusage" "$lib/release-version" "$lib/scheduler" "$lib/cron-run" "$units/device-usage-keeper.service" "$units/device-usage-keeper.timer")
cleanup() {
  result=$?
  if $updating && ((result != 0)); then
    set +e
    rollback_failed=false
    for i in "${!files[@]}"; do
      if [[ -e "$tmp/old-$i" ]]; then cp -p "$tmp/old-$i" "${files[$i]}" || rollback_failed=true
      else rm -f "${files[$i]}" || rollback_failed=true; fi
    done
    if [[ "$scheduler" == cron ]]; then
      if $cron_changed; then crontab "$tmp/old-cron" || rollback_failed=true; fi
    else
      systemctl --user daemon-reload || rollback_failed=true
      if $timer_enabled; then systemctl --user enable device-usage-keeper.timer || rollback_failed=true
      else systemctl --user disable device-usage-keeper.timer || rollback_failed=true; fi
      if $timer_active; then systemctl --user start device-usage-keeper.timer || rollback_failed=true
      else systemctl --user stop device-usage-keeper.timer || rollback_failed=true; fi
    fi
    if $rollback_failed; then
      echo "Rollback incomplete; private recovery files retained at $tmp" >&2
      exit "$result"
    fi
    echo 'Installation failed; restored prior binaries, version and scheduler. Private state retained.' >&2
  fi
  rm -rf "$tmp"
  exit "$result"
}
trap cleanup EXIT
if [[ -n "${DEVICE_USAGE_BUNDLE:-}" ]]; then
  bundle="$(cd "$DEVICE_USAGE_BUNDLE" && pwd)"
  (cd "$bundle" && sha256sum --check SHA256SUMS >/dev/null)
  cp "$bundle/device-sync-linux-amd64" "$tmp/device-sync"
  cp "$bundle/collector.tgz" "$tmp/collector.tgz"
else
curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --max-time 120 --max-filesize 67108864 "https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/download/$DEVICE_USAGE_VERSION/device-sync-linux-amd64" -o "$tmp/device-sync"
printf '%s  %s\n' "$DEVICE_USAGE_SHA256" "$tmp/device-sync" | sha256sum -c - >/dev/null
curl --fail --silent --show-error --proto '=https' --max-time 120 --max-filesize 33554432 'https://registry.npmjs.org/@ccusage/ccusage-linux-x64/-/ccusage-linux-x64-20.0.43.tgz' -o "$tmp/collector.tgz"
fi
release_version=unknown
if [[ -n "${DEVICE_USAGE_BUNDLE:-}" && -f "$bundle/RELEASE_VERSION" ]]; then
  release_version="$(<"$bundle/RELEASE_VERSION")"
  [[ "$release_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid bundle release version' >&2; exit 1; }
elif [[ -z "${DEVICE_USAGE_BUNDLE:-}" ]]; then
  release_version="$DEVICE_USAGE_VERSION"
fi
# SHA512 of the pinned official npm platform archive (not a version-string check).
printf '%s  %s\n' '0b8e1afcb6b2f37dd29a3776615dd27aa1821820634d7bacf353c777de3d7cebfa6db830f714940dd9492332b5453e0c765a4e8bc5669a707275e331f5d9699e' "$tmp/collector.tgz" | sha512sum -c - >/dev/null
tar -xOf "$tmp/collector.tgz" package/bin/ccusage > "$tmp/ccusage"
chmod 700 "$tmp/device-sync" "$tmp/ccusage"
[[ "$("$tmp/ccusage" --version)" == 'ccusage 20.0.43' ]] || { echo 'Collector version mismatch.' >&2; exit 1; }
mkdir -p "$bin" "$lib" "$units"
# Snapshot before mutation. Cron runs already in flight keep their open executable;
# device-sync's existing state lock prevents overlapping syncs.
for i in "${!files[@]}"; do [[ ! -e "${files[$i]}" ]] || cp -p "${files[$i]}" "$tmp/old-$i"; done
if [[ "$scheduler" == cron ]]; then
  read_cron "$tmp/old-cron"
  strip_cron "$tmp/old-cron" "$tmp/clean-cron"
else
  if systemctl --user is-active --quiet device-usage-keeper.timer; then timer_active=true; fi
  if systemctl --user is-enabled --quiet device-usage-keeper.timer; then timer_enabled=true; fi
fi
updating=true
if [[ "$scheduler" == cron ]]; then
  cron_changed=true
  crontab "$tmp/clean-cron"
elif [[ "$mode" == update ]]; then
  systemctl --user stop device-usage-keeper.timer device-usage-keeper.service
fi
install -m 700 "$tmp/device-sync" "$bin/device-sync.new"
install -m 700 "$tmp/ccusage" "$lib/ccusage.new"
mv "$bin/device-sync.new" "$bin/device-sync"
mv "$lib/ccusage.new" "$lib/ccusage"
if [[ "$mode" == install ]]; then "$bin/device-sync" configure; fi
if [[ "$mode" == update ]]; then printf 'Saved source consent preserved. Legacy configurations without consent cannot sync; run install.sh sources to approve roots.\n'; fi
if [[ "$scheduler" == systemd ]]; then
printf '%s\n' '[Unit]' 'Description=Upload coding-agent aggregate snapshots' '[Service]' 'Type=oneshot' 'ExecStart=%h/.local/bin/device-sync sync' 'UMask=0077' 'TimeoutStartSec=15min' 'NoNewPrivileges=true' 'PrivateTmp=true' > "$units/device-usage-keeper.service"
printf '%s\n' '[Unit]' 'Description=Collect device usage every 15 minutes' '[Timer]' 'OnBootSec=2min' 'OnUnitActiveSec=15min' 'RandomizedDelaySec=60' '[Install]' 'WantedBy=timers.target' > "$units/device-usage-keeper.timer"
systemctl --user daemon-reload
systemctl --user enable --now device-usage-keeper.timer
else
  {
    printf '#!/bin/sh\n# Do not record collector output: it may contain private data.\numask 077\n'
    printf 'HOME=%s\nexport HOME\nPATH=/usr/local/bin:/usr/bin:/bin\nexport PATH\nunset DEVICE_USAGE_STATE\n' "$(shell_quote "$HOME")"
    printf 'cd "$HOME" || exit 1\nif "$HOME/.local/bin/device-sync" sync >/dev/null 2>&1; then\n  printf "Sync succeeded\\n"\nelse\n  printf "Sync failed; run device-sync status\\n"\nfi > "$HOME/.local/lib/device-usage-keeper/cron-status"\n'
  } > "$lib/cron-run"
  chmod 700 "$lib/cron-run"
  # Encode path bytes as octal: cron treats percent specially even in quotes.
  # No literal HOME characters reach cron's parser or shell syntax.
  encoded=''
  path="$lib/cron-run"
  LC_ALL=C
  for ((i=0; i<${#path}; i++)); do
    printf -v byte '\\%03o' "'${path:i:1}"
    encoded+="$byte"
  done
  command_line="/bin/sh \"\$(printf '$encoded')\""
  printf '*/15 * * * * %s >/dev/null 2>&1 %s\n' "$command_line" "$marker" >> "$tmp/clean-cron"
  crontab "$tmp/clean-cron"
fi
printf '%s\n' "$scheduler" > "$lib/scheduler"
printf '%s\n' "$release_version" > "$lib/release-version.new"
mv "$lib/release-version.new" "$lib/release-version"
updating=false
printf 'Installed with %s. Use install.sh status.\n' "$scheduler"
if [[ "$scheduler" == cron ]]; then
  printf 'Keep cron/crond running through your container supervisor, including after restarts. No services were installed or started.\n'
else printf 'Collection runs in the user session; lingering is optional and unchanged.\n'; fi
