#!/usr/bin/env bash
# Feature-level installer mocks. Uses only a temporary HOME; no real scheduler.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home"; mkdir -p "$HOME/.local/bin" "$tmp/bin"
export SOURCE_TEST_LOG="$tmp/calls"
unset DEVICE_USAGE_STATE DEVICE_USAGE_BUNDLE
cat > "$HOME/.local/bin/device-sync" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$SOURCE_TEST_LOG"
exit "${SOURCE_TEST_EXIT:-0}"
EOF
chmod +x "$HOME/.local/bin/device-sync"
bash "$root/install.sh" sources
test "$(<"$SOURCE_TEST_LOG")" = sources
if SOURCE_TEST_EXIT=23 bash "$root/install.sh" sources; then echo 'Linux swallowed selection failure'; exit 1; fi
cat > "$tmp/bin/uname" <<'EOF'
#!/usr/bin/env bash
case "$1" in -s) echo Darwin ;; -m) echo arm64 ;; *) exit 1 ;; esac
EOF
chmod +x "$tmp/bin/uname"
: > "$SOURCE_TEST_LOG"
PATH="$tmp/bin:$PATH" bash "$root/install.sh" sources
test "$(<"$SOURCE_TEST_LOG")" = sources
if PATH="$tmp/bin:$PATH" SOURCE_TEST_EXIT=23 bash "$root/install.sh" sources; then echo 'macOS swallowed selection failure'; exit 1; fi
echo 'PASS: Linux and mocked Darwin source entrypoints and failure propagation'
if [[ -n "${PWSH:-}" ]] || command -v pwsh >/dev/null; then
 pwsh="${PWSH:-$(command -v pwsh)}"
 cp "$HOME/.local/bin/device-sync" "$HOME/.local/bin/device-sync.exe"
 export SOURCE_TEST_HOME="$HOME" SOURCE_TEST_INSTALLER="$root/install.ps1"
 cat > "$tmp/windows.ps1" <<'EOF'
$ErrorActionPreference = 'Stop'
# Only the source-selection entrypoint is run: no Windows ACL/task APIs.
Set-Variable -Name HOME -Value $env:SOURCE_TEST_HOME -Force
$env:OS = 'Windows_NT'
& $env:SOURCE_TEST_INSTALLER -Mode sources
exit $LASTEXITCODE
EOF
 : > "$SOURCE_TEST_LOG"
 "$pwsh" -NoProfile -File "$tmp/windows.ps1"
 test "$(<"$SOURCE_TEST_LOG")" = sources
 if SOURCE_TEST_EXIT=23 "$pwsh" -NoProfile -File "$tmp/windows.ps1"; then echo 'Windows swallowed selection failure'; exit 1; fi
 echo 'PASS: PowerShell Windows-mode source entrypoint and failure propagation (mock executable, no native APIs)'
else
 echo 'SKIP: Windows-mode entrypoint requires pwsh (set PWSH=/path/to/pwsh)'
fi
