#!/usr/bin/env bash
set -euo pipefail
umask 077
[[ "$(uname -s)" == Darwin ]] || { echo 'macOS required' >&2; exit 1; }
[[ -z "${DEVICE_USAGE_STATE:-}" ]] || { echo 'Custom state requires manual scheduling' >&2; exit 1; }
mode="${1:-menu}"
label=com.device-usage-keeper.sync
unit="$HOME/Library/LaunchAgents/$label.plist"
exe="$HOME/.local/bin/device-sync"
lib="$HOME/.local/lib/device-usage-keeper"
state="$HOME/.local/state/device-usage-keeper"
domain="gui/$(id -u)"
if [[ "$mode" == menu ]]; then
 if [[ -d "$state" || -e "$exe" || -e "$lib/ccusage" || -e "$unit" ]]; then
  echo 'Existing or partial installation found. Update requires a complete installation; partial state needs backup recovery.'
 fi
 printf 'Choose: install / update / sources / status / uninstall / cancel: ' >/dev/tty
 read -r mode </dev/tty
fi
case "$mode" in
 cancel|'') exit 0 ;;
 sources) exec "$exe" sources ;;
 status) launchctl print "$domain/$label"; exec "$exe" status ;;
 uninstall) launchctl bootout "$domain/$label" 2>/dev/null || true; rm -f "$unit" "$exe" "$lib/ccusage"; echo 'Uninstalled; private state retained. Revoke device in dashboard.'; exit ;;
 install|update) ;;
 *) echo 'Usage: install.sh [install|update|sources|status|uninstall]' >&2; exit 1 ;;
esac
bundle="${DEVICE_USAGE_BUNDLE:?Set DEVICE_USAGE_BUNDLE to the trusted macOS bundle directory}"
case "$(uname -m)" in arm64) arch=arm64 ;; x86_64) arch=amd64 ;; *) echo 'Unsupported architecture' >&2; exit 1 ;; esac
(cd "$bundle" && shasum -a 256 -c SHA256SUMS >/dev/null)
if [[ "$mode" == install && ( -e "$state/config.json" || -e "$state/state.json" || -e "$exe" || -e "$lib/ccusage" || -e "$unit" ) ]]; then echo 'Existing or partial installation; use update for complete installs, otherwise restore a backup. Do not delete private state.' >&2; exit 1; fi
if [[ "$mode" == update ]]; then
 for f in "$exe" "$lib/ccusage" "$unit" "$state/config.json" "$state/state.json"; do [[ -f "$f" ]] || { echo 'Incomplete installation; restore backup' >&2; exit 1; }; done
fi
tmp="$(mktemp -d)"; stopped=false
cleanup() {
 result=$?
 if $stopped && (( result != 0 )); then
  cp -p "$tmp/old-sync" "$exe"; cp -p "$tmp/old-collector" "$lib/ccusage"; cp -p "$tmp/old-unit" "$unit"
  launchctl bootstrap "$domain" "$unit" || true
  echo 'Update failed; restored previous files. State retained.' >&2
 fi
 rm -rf "$tmp"; exit "$result"
}
trap cleanup EXIT
cp "$bundle/device-sync-darwin-$arch" "$tmp/device-sync"
tar -xOf "$bundle/collector.tgz" package/bin/ccusage > "$tmp/ccusage"
chmod 700 "$tmp/device-sync" "$tmp/ccusage"
[[ "$("$tmp/ccusage" --version)" == 'ccusage 20.0.43' ]] || exit 1
mkdir -p "$HOME/.local/bin" "$lib" "$(dirname "$unit")"
if [[ "$mode" == update ]]; then
 cp -p "$exe" "$tmp/old-sync"; cp -p "$lib/ccusage" "$tmp/old-collector"; cp -p "$unit" "$tmp/old-unit"
 launchctl bootout "$domain/$label"; stopped=true
fi
install -m 700 "$tmp/device-sync" "$exe.new"; mv "$exe.new" "$exe"
install -m 700 "$tmp/ccusage" "$lib/ccusage.new"; mv "$lib/ccusage.new" "$lib/ccusage"
if [[ "$mode" == install ]]; then "$exe" configure; fi
if [[ "$mode" == update ]]; then echo 'Saved source consent preserved. Legacy configurations without consent cannot sync; run install.sh sources to approve roots.'; fi
# Escape paths for XML; launchd ProgramArguments never invokes a shell.
escaped="$(printf '%s' "$exe" | sed 's/\&/\&amp;/g;s/</\&lt;/g;s/>/\&gt;/g')"
printf '%s\n' '<?xml version="1.0" encoding="UTF-8"?>' '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' '<plist version="1.0"><dict>' "<key>Label</key><string>$label</string>" "<key>ProgramArguments</key><array><string>$escaped</string><string>sync</string></array>" '<key>StartInterval</key><integer>900</integer><key>RunAtLoad</key><true/><key>Umask</key><integer>63</integer></dict></plist>' > "$unit"
plutil -lint "$unit"
launchctl bootstrap "$domain" "$unit"
stopped=false
echo 'Installed LaunchAgent. Runs at login and every 15 minutes while logged in. Use ~/.local/bin/device-sync status.'
