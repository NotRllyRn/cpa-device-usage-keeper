#!/bin/sh
# Hosted entrypoint; offline installers remain usable independently.
set -eu
umask 077
fail() { printf '%s\n' "$*" >&2; exit 1; }
version=${DEVICE_USAGE_VERSION:-}
case "$version" in *[!v0-9.]*|'') fail 'Invalid stable release tag.' ;; esac
printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || fail 'Set DEVICE_USAGE_VERSION to an approved stable release tag (vX.Y.Z).'
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
asset="device-usage-$version-$target.zip"
base="https://github.com/NotRllyRn/cpa-device-usage-keeper/releases/download/$version"
fetch() {
 curl --fail --silent --show-error --location --max-redirs 5 --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 20 --max-time 180 --max-filesize "$3" "$base/$1" -o "$2"
}
fetch "$asset.sha256" "$tmp/checksum" 1024
fetch "$asset" "$tmp/archive.zip" 134217728
python3 - "$tmp" "$asset" "$target" <<'PY'
import hashlib, pathlib, re, stat, sys, zipfile
root, asset, target = sys.argv[1:]
root = pathlib.Path(root)
manifest = (root / 'checksum').read_text()
if not re.fullmatch(r'[a-f0-9]{64}  ' + re.escape(asset) + r'\n', manifest):
    raise SystemExit('Invalid release checksum manifest')
archive = root / 'archive.zip'
if archive.stat().st_size > 134217728 or hashlib.sha256(archive.read_bytes()).hexdigest() != manifest[:64]:
    raise SystemExit('Release checksum mismatch')
expected = {'install.sh', 'install-macos.sh', 'install.ps1', 'LICENSE', 'BUILD.txt', 'SHA256SUMS', 'collector.tgz', 'device-sync-' + target}
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
PY
DEVICE_USAGE_BUNDLE="$tmp/bundle" bash "$tmp/bundle/install.sh" "$@" <&3
