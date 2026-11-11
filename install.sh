#!/usr/bin/env bash
# Device installer: Linux x86_64 with a user systemd instance. No root/npm needed.
set -euo pipefail
umask 077
mode="${1:-install}"
[[ -z "${DEVICE_USAGE_STATE:-}" ]] || { echo 'Installer supports only the default state directory; custom state requires manual scheduling.' >&2; exit 1; }
bin="$HOME/.local/bin"
lib="$HOME/.local/lib/device-usage-keeper"
units="$HOME/.config/systemd/user"
case "$mode" in
  status) systemctl --user status device-usage-keeper.timer --no-pager; exec "$bin/device-sync" status ;;
  uninstall)
    systemctl --user disable --now device-usage-keeper.timer
    systemctl --user stop device-usage-keeper.service
    rm -f "$units/device-usage-keeper.timer" "$units/device-usage-keeper.service"
    systemctl --user daemon-reload
    rm -f "$bin/device-sync" "$lib/ccusage"
    printf 'Removed binaries and scheduler. Private state retained for safe revisions and retry. Revoke this device in the dashboard.\n'
    exit 0 ;;
  install|update) ;;
  *) printf 'Usage: install.sh [install|update|status|uninstall]\n' >&2; exit 1 ;;
esac
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { echo 'Only Linux x86_64 is supported.' >&2; exit 1; }
for tool in curl sha256sum sha512sum tar systemctl; do command -v "$tool" >/dev/null || { echo "Missing $tool" >&2; exit 1; }; done
systemctl --user show-environment >/dev/null || { echo 'A working user systemd instance is required.' >&2; exit 1; }
# Releases are intentionally not published yet. Use an explicitly approved
# immutable release version and its SHA256 from a trusted release channel.
if [[ -z "${DEVICE_USAGE_BUNDLE:-}" ]]; then
: "${DEVICE_USAGE_VERSION:?Set DEVICE_USAGE_VERSION to an approved release tag}"
: "${DEVICE_USAGE_SHA256:?Set DEVICE_USAGE_SHA256 to the trusted device-sync-linux-amd64 SHA256}"
[[ "$DEVICE_USAGE_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ && "$DEVICE_USAGE_SHA256" =~ ^[a-fA-F0-9]{64}$ ]] || { echo 'Invalid version/checksum.' >&2; exit 1; }
fi
state="$HOME/.local/state/device-usage-keeper"
if [[ "$mode" == install && ( -e "$state/config.json" || -e "$state/state.json" ) ]]; then echo 'Already configured; use update.' >&2; exit 1; fi
if [[ "$mode" == update ]]; then
  for file in "$state/config.json" "$state/state.json" "$bin/device-sync" "$lib/ccusage" "$units/device-usage-keeper.service" "$units/device-usage-keeper.timer"; do
    [[ -f "$file" ]] || { echo 'Incomplete installation; restore from backup before update.' >&2; exit 1; }
  done
fi
tmp="$(mktemp -d)"; updating=false; timer_active=false
cleanup() {
  result=$?
  if $updating && ((result != 0)); then
    cp -p "$tmp/old-sync" "$bin/device-sync"
    cp -p "$tmp/old-collector" "$lib/ccusage"
    cp -p "$tmp/old-service" "$units/device-usage-keeper.service"
    cp -p "$tmp/old-timer" "$units/device-usage-keeper.timer"
    systemctl --user daemon-reload
    if $timer_active; then systemctl --user start device-usage-keeper.timer; fi
    echo 'Update failed; restored prior binaries and units. State retained.' >&2
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
# SHA512 of the pinned official npm platform archive (not a version-string check).
printf '%s  %s\n' '0b8e1afcb6b2f37dd29a3776615dd27aa1821820634d7bacf353c777de3d7cebfa6db830f714940dd9492332b5453e0c765a4e8bc5669a707275e331f5d9699e' "$tmp/collector.tgz" | sha512sum -c - >/dev/null
tar -xOf "$tmp/collector.tgz" package/bin/ccusage > "$tmp/ccusage"
chmod 700 "$tmp/device-sync" "$tmp/ccusage"
[[ "$("$tmp/ccusage" --version)" == 'ccusage 20.0.43' ]] || { echo 'Collector version mismatch.' >&2; exit 1; }
mkdir -p "$bin" "$lib" "$units"
# Stop the uploader before replacing either executable; state is preserved.
if [[ "$mode" == update ]]; then
  cp -p "$bin/device-sync" "$tmp/old-sync"
  cp -p "$lib/ccusage" "$tmp/old-collector"
  cp -p "$units/device-usage-keeper.service" "$tmp/old-service"
  cp -p "$units/device-usage-keeper.timer" "$tmp/old-timer"
  if systemctl --user is-active --quiet device-usage-keeper.timer; then timer_active=true; fi
  updating=true
  systemctl --user stop device-usage-keeper.timer device-usage-keeper.service
fi
install -m 700 "$tmp/device-sync" "$bin/device-sync.new"
install -m 700 "$tmp/ccusage" "$lib/ccusage.new"
mv "$bin/device-sync.new" "$bin/device-sync"
mv "$lib/ccusage.new" "$lib/ccusage"
if [[ "$mode" == install ]]; then "$bin/device-sync" configure; fi
printf '%s\n' '[Unit]' 'Description=Upload coding-agent aggregate snapshots' '[Service]' 'Type=oneshot' 'ExecStart=%h/.local/bin/device-sync sync' 'UMask=0077' 'TimeoutStartSec=15min' 'NoNewPrivileges=true' 'PrivateTmp=true' > "$units/device-usage-keeper.service"
printf '%s\n' '[Unit]' 'Description=Collect device usage every 15 minutes' '[Timer]' 'OnBootSec=2min' 'OnUnitActiveSec=15min' 'RandomizedDelaySec=60' '[Install]' 'WantedBy=timers.target' > "$units/device-usage-keeper.timer"
systemctl --user daemon-reload
systemctl --user enable --now device-usage-keeper.timer
updating=false
printf 'Installed. Use device-sync status. Collection runs in the user session; lingering is optional and not changed by this installer.\n'
