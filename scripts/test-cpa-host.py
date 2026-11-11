#!/usr/bin/env python3
"""Run inside an isolated network namespace. Synthetic state only, no install."""
import json
import os
import pathlib
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request

if len(sys.argv) != 3:
    sys.exit('usage: test-cpa-host.py CPA_BINARY PLUGIN_SO (inside isolated net namespace)')
host, plugin = map(os.path.abspath, sys.argv[1:])
def port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]
def get(url):
    with urllib.request.urlopen(url, timeout=1) as response:
        return response.read()
with tempfile.TemporaryDirectory(prefix='device-cpa-test-') as root:
    root = pathlib.Path(root)
    for name in ('home', 'auth', 'plugins', 'data'):
        (root / name).mkdir(mode=0o700)
    shutil.copyfile(plugin, root / 'plugins/device-usage-keeper.so')
    proxy_port, device_port = port(), port()
    (root / 'config.yaml').write_text(f'''config-version: 8
server:
  host: "127.0.0.1"
  port: {proxy_port}
  discovery:
    enabled: false
  kv-store:
    disabled: true
management:
  allow-remote: false
  secret-key: ""
  disable-control-panel: true
  disable-auto-update-panel: true
oauth:
  auth-dir: "{root}/auth"
plugins:
  enabled: true
  dir: "{root}/plugins"
  configs:
    device-usage-keeper:
      enabled: true
      priority: 0
      listen: "127.0.0.1:{device_port}"
      data_dir: "{root}/data"
      dashboard_url: "https://dashboard.invalid"
''')
    for iteration in range(2):
        with (root / 'host.log').open('wb') as log:
            process = subprocess.Popen([host, '-config', str(root / 'config.yaml'), '-local-model'], cwd=root,
                env={'PATH': os.environ['PATH'], 'HOME': str(root / 'home'), 'TMPDIR': str(root)}, stdout=log, stderr=log)
            try:
                for attempt in range(100):
                    try:
                        assert json.loads(get(f'http://127.0.0.1:{device_port}/healthz'))['status'] == 'ok'
                        break
                    except (OSError, AssertionError):
                        if process.poll() is not None:
                            raise RuntimeError('CPA exited before plugin readiness')
                        time.sleep(.1)
                else:
                    raise RuntimeError('plugin readiness timeout')
                assert b'https://dashboard.invalid' in get(f'http://127.0.0.1:{proxy_port}/v0/resource/plugins/device-usage-keeper/open')
                assert b'/assets/' in get(f'http://127.0.0.1:{device_port}/')
            except Exception:
                # This isolated host contains no user credentials/data. Print only
                # lines identifying plugin errors, never the generated token file.
                for line in (root / 'host.log').read_text(errors='replace').splitlines():
                    if 'plugin' in line.lower():
                        print(line)
                raise
            finally:
                process.terminate()
                try:
                    process.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    process.kill(); process.wait()
                    raise RuntimeError('CPA shutdown timeout')
            for p in (proxy_port, device_port):
                with socket.socket() as s:
                    assert s.connect_ex(('127.0.0.1', p)) != 0, 'listener survived shutdown'
    print('PASS: real CPA host register/resource/dashboard/shutdown/restart with isolated temporary state')
