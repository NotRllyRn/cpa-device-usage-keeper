#!/usr/bin/env bash
# Builds artifacts only. Does not install, enroll, collect usage, or publish.
set -euo pipefail
[[ "$(uname -s)/$(uname -m)" == Linux/x86_64 && "$(go env GOOS)/$(go env GOARCH)" == linux/amd64 ]] || { echo 'Test bundles require native Linux/amd64 and a matching Go target.' >&2; exit 1; }
root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:?Usage: bash scripts/build-test-bundle.sh /absolute/new/output-directory}"
[[ "$out" == /* && ! -e "$out" ]] || { echo 'Use a new absolute output directory.' >&2; exit 1; }
umask 077
mkdir -p "$out"
cd "$root/web"
npm ci --ignore-scripts --no-audit --no-fund --fetch-retries=0 --fetch-timeout=20000
npm run typecheck
npx vite build --config vite.device.config.js
touch device-dist/.gitkeep
cd "$root"
CGO_ENABLED=1 go build -trimpath -buildmode=c-shared -o "$out/device-usage-keeper.so" ./cmd/device-plugin
go build -trimpath -o "$out/device-sync-linux-amd64" ./cmd/device-sync
curl --fail --silent --show-error --proto '=https' --max-time 120 --max-filesize 33554432 'https://registry.npmjs.org/@ccusage/ccusage-linux-x64/-/ccusage-linux-x64-20.0.43.tgz' -o "$out/collector.tgz"
printf '%s  %s\n' '0b8e1afcb6b2f37dd29a3776615dd27aa1821820634d7bacf353c777de3d7cebfa6db830f714940dd9492332b5453e0c765a4e8bc5669a707275e331f5d9699e' "$out/collector.tgz" | sha512sum --check - >/dev/null
cp install.sh LICENSE "$out/"
cp docs/native/bridge-LICENSE "$out/bridge-LICENSE"
cp /dev/null "$out/BUILD.txt"
printf 'Source commit: %s\nBuilt with: %s\nPlatform: %s/%s\n' "$(git rev-parse HEAD)" "$(go version)" "$(go env GOOS)" "$(go env GOARCH)" > "$out/BUILD.txt"
git diff --quiet || printf 'Working tree contains uncommitted changes\n' >> "$out/BUILD.txt"
(cd "$out" && sha256sum device-usage-keeper.so device-sync-linux-amd64 collector.tgz install.sh LICENSE bridge-LICENSE BUILD.txt > SHA256SUMS)
printf 'Local test bundle: %s\nTransfer it privately through a trusted channel. On a test device: DEVICE_USAGE_BUNDLE=/path/to/bundle bash /path/to/bundle/install.sh\n' "$out"
