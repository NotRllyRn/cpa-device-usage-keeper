#!/usr/bin/env bash
# Isolated mocked systemd and synthetic binaries; never touches the real user.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home"; mkdir -p "$HOME" "$tmp/bin" "$tmp/bundle/package/bin"
export PATH="$tmp/bin:$PATH"
cat > "$tmp/bin/systemctl" <<'EOF'
#!/usr/bin/env bash
if [[ "${FAIL_ENABLE:-}" == 1 && "$*" == *enable* ]]; then exit 1; fi
exit 0
EOF
cat > "$tmp/bin/sha512sum" <<'EOF'
#!/usr/bin/env bash
# Synthetic collector archive; real SHA512 is checked by the bundle builder.
cat >/dev/null
EOF
cat > "$tmp/bundle/device-sync-linux-amd64" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == configure ]]; then
 mkdir -p "$HOME/.local/state/device-usage-keeper"
 printf '{}\n' > "$HOME/.local/state/device-usage-keeper/config.json"
 printf '{}\n' > "$HOME/.local/state/device-usage-keeper/state.json"
fi
EOF
cat > "$tmp/bundle/package/bin/ccusage" <<'EOF'
#!/usr/bin/env bash
printf 'ccusage 20.0.43\n'
EOF
chmod +x "$tmp/bin/"* "$tmp/bundle/device-sync-linux-amd64" "$tmp/bundle/package/bin/ccusage"
printf 'v1.0.0\n' > "$tmp/bundle/RELEASE_VERSION"
(cd "$tmp/bundle" && tar czf collector.tgz package && sha256sum device-sync-linux-amd64 collector.tgz RELEASE_VERSION > SHA256SUMS)
export DEVICE_USAGE_BUNDLE="$tmp/bundle"
bash "$root/install.sh" install >/dev/null
test -f "$HOME/.config/systemd/user/device-usage-keeper.timer"
test "$(<"$HOME/.local/lib/device-usage-keeper/release-version")" = v1.0.0
if bash "$root/install.sh" install > /dev/null 2>&1; then echo 'Reinstall should reject existing state'; exit 1; fi
before="$(sha256sum "$HOME/.local/bin/device-sync")"
printf 'v1.2.3\n' > "$tmp/bundle/RELEASE_VERSION"
(cd "$tmp/bundle" && sha256sum device-sync-linux-amd64 collector.tgz RELEASE_VERSION > SHA256SUMS)
if FAIL_ENABLE=1 bash "$root/install.sh" update > /dev/null 2>&1; then echo 'Update should fail'; exit 1; fi
test "$(sha256sum "$HOME/.local/bin/device-sync")" = "$before"
test "$(<"$HOME/.local/lib/device-usage-keeper/release-version")" = v1.0.0
bash "$root/install.sh" update >/dev/null
test "$(<"$HOME/.local/lib/device-usage-keeper/release-version")" = v1.2.3
bash "$root/install.sh" uninstall >/dev/null
test ! -e "$HOME/.local/bin/device-sync"
test -f "$HOME/.local/state/device-usage-keeper/state.json"
if GOOS=windows bash "$root/scripts/build-test-bundle.sh" "$tmp/invalid" >/dev/null 2>&1; then echo 'Unsupported build should fail'; exit 1; fi
test ! -e "$tmp/invalid"
echo 'PASS: Linux install/reinstall/update rollback/uninstall/state retention and release markers'
# macOS scheduling is mocked; exercise its marker commit/rollback boundary too.
export HOME="$tmp/mac-home"; mkdir -p "$HOME"
cp "$tmp/bundle/device-sync-linux-amd64" "$tmp/bundle/device-sync-darwin-arm64"
(cd "$tmp/bundle" && sha256sum device-sync-darwin-arm64 collector.tgz RELEASE_VERSION > SHA256SUMS)
cat > "$tmp/bin/uname" <<'EOF'
#!/bin/sh
case "$1" in -s) echo Darwin ;; -m) echo arm64 ;; esac
EOF
cat > "$tmp/bin/launchctl" <<'EOF'
#!/bin/sh
exit 0
EOF
cat > "$tmp/bin/plutil" <<'EOF'
#!/bin/sh
exit "${FAIL_PLUTIL:-0}"
EOF
chmod +x "$tmp/bin/uname" "$tmp/bin/launchctl" "$tmp/bin/plutil"
bash "$root/install-macos.sh" install >/dev/null
test "$(<"$HOME/.local/lib/device-usage-keeper/release-version")" = v1.2.3
printf 'v1.3.0\n' > "$tmp/bundle/RELEASE_VERSION"
(cd "$tmp/bundle" && sha256sum device-sync-darwin-arm64 collector.tgz RELEASE_VERSION > SHA256SUMS)
if FAIL_PLUTIL=1 bash "$root/install-macos.sh" update >/dev/null 2>&1; then echo 'macOS update should fail'; exit 1; fi
test "$(<"$HOME/.local/lib/device-usage-keeper/release-version")" = v1.2.3
bash "$root/install-macos.sh" update >/dev/null
test "$(<"$HOME/.local/lib/device-usage-keeper/release-version")" = v1.3.0
bash "$root/install-macos.sh" uninstall >/dev/null
test ! -e "$HOME/.local/lib/device-usage-keeper/release-version"
test -f "$HOME/.local/state/device-usage-keeper/state.json"
echo 'PASS: mocked macOS release marker install/update/rollback/uninstall'
