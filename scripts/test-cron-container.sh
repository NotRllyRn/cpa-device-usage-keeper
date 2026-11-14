#!/usr/bin/env bash
# Disposable Alpine container only. Downloads test packages; no host crontab access.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
docker run --rm -i -v "$root/install.sh:/test/install.sh:ro" alpine:3.21 sh -s <<'CONTAINER'
set -eu
apk add --no-cache bash curl coreutils procps tar >/dev/null
export HOME="/tmp/home space % ' dollar\$ slash\\ end"
mkdir -p "$HOME" /fixture/package/bin /mock
cat > /mock/sha512sum <<'EOF'
#!/bin/sh
cat >/dev/null
EOF
cat > /fixture/device-sync-linux-amd64 <<'EOF'
#!/bin/sh
case "$1" in
 configure)
  mkdir -p "$HOME/.local/state/device-usage-keeper"
  echo '{}' > "$HOME/.local/state/device-usage-keeper/config.json"
  echo '{}' > "$HOME/.local/state/device-usage-keeper/state.json" ;;
 sync)
  printf '%s\n%s\n' "$HOME" "$PWD" > "$HOME/smoke-result"
  echo 'synthetic sensitive output' >&2 ;;
esac
EOF
cat > /fixture/package/bin/ccusage <<'EOF'
#!/bin/sh
echo 'ccusage 20.0.43'
EOF
chmod +x /mock/sha512sum /fixture/device-sync-linux-amd64 /fixture/package/bin/ccusage
(cd /fixture && tar czf collector.tgz package && echo v0.1.1 > RELEASE_VERSION && sha256sum device-sync-linux-amd64 collector.tgz RELEASE_VERSION > SHA256SUMS)
export PATH="/mock:$PATH" DEVICE_USAGE_BUNDLE=/fixture
# BusyBox crond is intentionally started only inside this disposable test container.
crond -f -l 8 >/tmp/daemon.log 2>&1 &
printf '# unrelated entry\n' | crontab -
bash /test/install.sh install
# Accelerate this isolated smoke test, leaving the generated command unchanged.
crontab -l | sed 's|^\*/15 |* |' > /tmp/accelerated
crontab /tmp/accelerated
for i in $(seq 1 80); do
  [ ! -f "$HOME/smoke-result" ] || break
  sleep 1
done
[ "$(head -n 1 "$HOME/smoke-result")" = "$HOME" ]
[ "$(tail -n 1 "$HOME/smoke-result")" = "$HOME" ]
[ "$(cat "$HOME/.local/lib/device-usage-keeper/cron-status")" = 'Sync succeeded' ]
bash /test/install.sh uninstall
[ "$(crontab -l)" = '# unrelated entry' ]
[ -f "$HOME/.local/state/device-usage-keeper/state.json" ]
echo 'PASS: real isolated BusyBox crond executes generated command with special HOME; uninstall preserves unrelated crontab and state'
CONTAINER
