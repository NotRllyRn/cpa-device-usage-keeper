#!/bin/sh
# Hosted entrypoint; offline installers remain usable independently.
set -eu
umask 077
fail() { printf '%s\n' "$*" >&2; exit 1; }
version=${DEVICE_USAGE_VERSION:-}

case "$(uname -s)/$(uname -m)" in
 Linux/x86_64) target=linux-amd64 ;;
 Darwin/x86_64) target=darwin-amd64 ;;
 Darwin/arm64) target=darwin-arm64 ;;
 *) fail 'Supported: Linux x86_64, macOS x86_64/arm64.' ;;
esac
for tool in curl python3 bash; do command -v "$tool" >/dev/null || fail "Missing prerequisite: $tool"; done
# Open before downloading: piped stdin belongs to this script, never to prompts.
exec 3</dev/tty || fail 'An interactive terminal is required.'
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' 0
trap 'exit 130' INT
trap 'exit 143' TERM
download() {
 curl --fail --silent --show-error --location --max-redirs 5 --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 20 --max-time 180 --max-filesize "$3" "$1" -o "$2"
}
if [ -z "$version" ]; then
 download 'https://api.github.com/repos/NotRllyRn/cpa-device-usage-keeper/releases/latest' "$tmp/latest.json" 1048576
 version=$(python3 - "$tmp/latest.json" <<'PY'
import json, pathlib, re, sys
p = pathlib.Path(sys.argv[1])
if p.stat().st_size > 1048576:
    raise SystemExit('Release response too large')
r = json.loads(p.read_text())
if not isinstance(r, dict) or r.get('draft') is not False or r.get('prerelease') is not False or not isinstance(r.get('tag_name'), str) or not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+', r['tag_name']):
    raise SystemExit('Invalid stable release response')
print(r['tag_name'])
PY
 )
fi
case "$version" in *[!v0-9.]*|'') fail 'Invalid stable release tag.' ;; esac
printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || fail 'Invalid stable release tag.'
# The CLI currently reports a development version, not a release identifier.
# A marker is written only after a successful bundle installation/update.
lib="$HOME/.local/lib/device-usage-keeper"
existing=false
for path in "$HOME/.local/state/device-usage-keeper" "$HOME/.local/bin/device-sync" "$lib" "$HOME/.config/systemd/user/device-usage-keeper.timer" "$HOME/.config/systemd/user/device-usage-keeper.service" "$HOME/Library/LaunchAgents/com.device-usage-keeper.sync.plist"; do
 if [ -e "$path" ]; then existing=true; fi
done
if $existing; then
 installed=$(python3 - "$lib/release-version" <<'PY'
import pathlib, re, sys
p = pathlib.Path(sys.argv[1])
try:
    value = p.read_text() if p.stat().st_size < 128 else ''
except (OSError, UnicodeError):
    value = ''
print(value.strip() if re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+\n?', value) else 'unknown')
PY
 )
 printf 'Installed: %s; available: %s\n' "$installed" "$version"
 if [ "$installed" != "$version" ]; then
  action=$(python3 - "$installed" "$version" <<'PY'
import sys
old, new = sys.argv[1:]
print('switch from unknown version' if old == 'unknown' else 'upgrade' if tuple(map(int, new[1:].split('.'))) > tuple(map(int, old[1:].split('.'))) else 'switch/downgrade')
PY
  )
  printf 'Confirm %s to %s? [y/N]: ' "$action" "$version" >/dev/tty
  read -r answer <&3
  case "$answer" in y|Y|yes|YES) set -- update ;; *) echo 'Cancelled; installation unchanged.'; exit 0 ;; esac
 fi
fi
asset="device-usage-$version-$target.zip"
base="https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/download/$version"
fetch() { download "$base/$1" "$2" "$3"; }
fetch "$asset.sha256" "$tmp/checksum" 1024
fetch "$asset" "$tmp/archive.zip" 134217728
python3 - "$tmp" "$asset" "$target" "$version" <<'PY'
import hashlib, pathlib, re, stat, sys, zipfile
root, asset, target, version = sys.argv[1:]
root = pathlib.Path(root)
manifest = (root / 'checksum').read_text()
if not re.fullmatch(r'[a-f0-9]{64}  ' + re.escape(asset) + r'\n', manifest):
    raise SystemExit('Invalid release checksum manifest')
archive = root / 'archive.zip'
if archive.stat().st_size > 134217728 or hashlib.sha256(archive.read_bytes()).hexdigest() != manifest[:64]:
    raise SystemExit('Release checksum mismatch')
expected = {'install.sh', 'install-macos.sh', 'install.ps1', 'LICENSE', 'BUILD.txt', 'RELEASE_VERSION', 'SHA256SUMS', 'collector.tgz', 'device-sync-' + target}
with zipfile.ZipFile(archive) as z:
    entries = z.infolist()
    if len(entries) != len(expected) or {i.filename for i in entries} != expected:
        raise SystemExit('Unexpected archive members')
    if sum(i.file_size for i in entries) > 268435456:
        raise SystemExit('Archive too large')
    dest = root / 'bundle'
    dest.mkdir(mode=0o700)
    for i in entries:
        mode = i.external_attr >> 16
        if i.is_dir() or (stat.S_IFMT(mode) not in (0, stat.S_IFREG)):
            raise SystemExit('Non-regular archive member')
        (dest / i.filename).write_bytes(z.read(i))
    if (dest / 'RELEASE_VERSION').read_text() != version + '\n':
        raise SystemExit('Bundle release version mismatch')
PY
DEVICE_USAGE_BUNDLE="$tmp/bundle" bash "$tmp/bundle/install.sh" "$@" <&3
