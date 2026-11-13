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
(cd "$tmp/bundle" && tar czf collector.tgz package && sha256sum device-sync-linux-amd64 collector.tgz > SHA256SUMS)
export DEVICE_USAGE_BUNDLE="$tmp/bundle"
bash "$root/install.sh" install >/dev/null
test -f "$HOME/.config/systemd/user/device-usage-keeper.timer"
if bash "$root/install.sh" install > /dev/null 2>&1; then echo 'Reinstall should reject existing state'; exit 1; fi
before="$(sha256sum "$HOME/.local/bin/device-sync")"
if FAIL_ENABLE=1 bash "$root/install.sh" update > /dev/null 2>&1; then echo 'Update should fail'; exit 1; fi
test "$(sha256sum "$HOME/.local/bin/device-sync")" = "$before"
bash "$root/install.sh" update >/dev/null
bash "$root/install.sh" uninstall >/dev/null
test ! -e "$HOME/.local/bin/device-sync"
test -f "$HOME/.local/state/device-usage-keeper/state.json"
if GOOS=windows bash "$root/scripts/build-test-bundle.sh" "$tmp/invalid" >/dev/null 2>&1; then echo 'Unsupported build should fail'; exit 1; fi
test ! -e "$tmp/invalid"
echo 'PASS: isolated installer install/reinstall/update failure rollback/update/uninstall/state retention/platform guard'
