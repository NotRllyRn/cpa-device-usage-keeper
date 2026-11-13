#!/usr/bin/env python3
"""Offline bootstrap tests: fake HTTPS, fake platform, real PTY and ZIP checks."""
import hashlib
import json
import os
from pathlib import Path
import pty
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent
with tempfile.TemporaryDirectory() as td:
    tmp = Path(td)
    tools = tmp / 'bin'
    tools.mkdir()
    def tool(name, text):
        p = tools / name
        p.write_text('#!/bin/sh\n' + text)
        p.chmod(0o700)
    tool('uname', 'if [ "$1" = -s ]; then echo "$TEST_OS"; else echo "$TEST_ARCH"; fi\n')
    tool('curl', '''out=; url=
while [ "$#" -gt 0 ]; do
 case "$1" in -o) shift; out=$1 ;; https://*) url=$1 ;; esac
 shift
done
printf '%s\n' "$url" >> "$FIXTURE/downloads"
case "$url" in */releases/latest) cp "$FIXTURE/latest.json" "$out" ;; *.sha256) cp "$FIXTURE/checksum" "$out" ;; *.zip) cp "$FIXTURE/archive.zip" "$out" ;; *) exit 1 ;; esac
''')
    env = dict(os.environ, PATH=str(tools)+':'+os.environ['PATH'], FIXTURE=td,
               DEVICE_USAGE_VERSION='v1.2.3', TEST_OS='Linux', TEST_ARCH='x86_64',
               MARKER=str(tmp/'invoked'), TMPDIR=td, HOME=str(tmp/'home'))
    def fixture(target, bad=None, version='v1.2.3'):
        asset = f'device-usage-v1.2.3-{target}.zip'
        with zipfile.ZipFile(tmp/'archive.zip', 'w') as z:
            for name in ['install.sh','install-macos.sh','install.ps1','LICENSE','BUILD.txt','RELEASE_VERSION','SHA256SUMS','collector.tgz','device-sync-'+target]:
                body = 'fixture'
                if name == 'install.sh':
                    body = '#!/bin/bash\n[[ -t 0 ]] || exit 44\nprintf "%s" "${1:-menu}" > "$MARKER"\n'
                if name == 'RELEASE_VERSION':
                    body = version+'\n'
                if bad == 'symlink' and name == 'LICENSE':
                    info = zipfile.ZipInfo(name)
                    info.create_system = 3
                    info.external_attr = 0o120777 << 16
                    z.writestr(info, '/tmp/escape')
                else:
                    z.writestr(name, body)
            if bad and bad != 'symlink':
                z.writestr(bad, 'bad')
        (tmp/'checksum').write_text(hashlib.sha256((tmp/'archive.zip').read_bytes()).hexdigest()+'  '+asset+'\n')
    def run(ok, answer=None, invoked=None):
        if invoked is None:
            invoked = ok
        (tmp/'downloads').write_text('')
        (tmp/'invoked').unlink(missing_ok=True)
        pid, fd = pty.fork()
        if pid == 0:
            os.execvpe('bash', ['bash','-c', 'cat "$1/bootstrap.sh" | sh', '_', str(ROOT)], env)
        if answer is not None:
            os.write(fd, (answer+'\n').encode())
        output = b''
        try:
            while True:
                chunk = os.read(fd, 4096)
                if not chunk:
                    break
                output += chunk
        except OSError:
            pass
        finally:
            os.close(fd)
        _, status = os.waitpid(pid, 0)
        assert (os.waitstatus_to_exitcode(status) == 0) == ok, output.decode()
        assert (tmp/'invoked').exists() == invoked
        assert not list(tmp.glob('tmp.*')), 'Bootstrap leaked temporary directory'
        return output.decode()
    for system, arch, target in [('Linux','x86_64','linux-amd64'),('Darwin','arm64','darwin-arm64'),('Darwin','x86_64','darwin-amd64')]:
        env.update(TEST_OS=system, TEST_ARCH=arch)
        fixture(target); run(True)
        fixture(target, '../escape'); run(False)
        fixture(target, '/absolute'); run(False)
        fixture(target, 'symlink'); run(False)
        fixture(target, version='v9.9.9'); run(False)
        fixture(target); (tmp/'checksum').write_text('0'*64+'  '+f'device-usage-v1.2.3-{target}.zip\n'); run(False)
    env.update(TEST_OS='Linux', TEST_ARCH='aarch64'); run(False)
    env.update(TEST_ARCH='x86_64', DEVICE_USAGE_VERSION='latest'); run(False)
    env['DEVICE_USAGE_VERSION'] = 'v1.2.3\n../../other'; run(False)
    # Latest is resolved once; malformed, draft and prerelease responses fail closed.
    env['DEVICE_USAGE_VERSION'] = ''
    fixture('linux-amd64')
    latest = {'tag_name': 'v1.2.3', 'draft': False, 'prerelease': False}
    for response in [[], {}, {**latest, 'tag_name':'dev'}, {**latest, 'draft':True}, {**latest, 'prerelease':True}, {**latest, 'prerelease':'false'}]:
        (tmp/'latest.json').write_text(json.dumps(response)); run(False)
    (tmp/'latest.json').write_text('not json'); run(False)
    (tmp/'latest.json').write_text(json.dumps(latest)); run(True)
    downloads = (tmp/'downloads').read_text().splitlines()
    assert len(downloads) == 3 and downloads[0].endswith('/releases/latest')
    assert all('/download/v1.2.3/' in u for u in downloads[1:])
    # Version comparisons are read-only until explicit consent, including legacy installs.
    lib = tmp/'home/.local/lib/device-usage-keeper'; lib.mkdir(parents=True)
    marker = lib/'release-version'
    for system, arch, target in [('Linux','x86_64','linux-amd64'),('Darwin','arm64','darwin-arm64')]:
        env.update(TEST_OS=system, TEST_ARCH=arch); fixture(target)
        for saved, label in [(None, 'unknown'), ('garbage', 'unknown'), ('v1.0.0\n','upgrade'), ('v2.0.0\n','switch/downgrade')]:
            marker.unlink(missing_ok=True)
            if saved is not None: marker.write_text(saved)
            out = run(True, 'n', False)
            assert label in out and 'unchanged' in out
            assert len((tmp/'downloads').read_text().splitlines()) == 1
            assert (marker.read_text() if marker.exists() else None) == saved
        marker.write_text('v1.0.0\n'); run(True, 'yes')
        assert (tmp/'invoked').read_text() == 'update'
        marker.write_text('v1.2.3\n'); out = run(True)
        assert 'Confirm' not in out and (tmp/'invoked').read_text() == 'menu'
    env.update(TEST_OS='Linux', TEST_ARCH='x86_64')
    # No controlling terminal: fail before downloading or invoking an installer.
    env['DEVICE_USAGE_VERSION'] = 'v1.2.3'
    result = subprocess.run(['sh', str(ROOT/'bootstrap.sh')], env=env,
                            stdin=subprocess.DEVNULL, capture_output=True, start_new_session=True)
    assert result.returncode != 0
    # Repeat menu recognizes partial state and cancel leaves it untouched.
    home = tmp/'home'; (home/'.local/bin').mkdir(parents=True)
    partial = home/'.local/bin/device-sync'; partial.write_text('partial')
    env.update(HOME=str(home), TEST_OS='Linux')
    pid, fd = pty.fork()
    if pid == 0:
        os.execvpe('bash', ['bash', str(ROOT/'install.sh')], env)
    os.write(fd, b'cancel\n'); output = b''
    try:
        while True:
            data = os.read(fd, 4096)
            if not data: break
            output += data
    except OSError:
        pass
    os.close(fd)
    _, status = os.waitpid(pid, 0)
    assert os.waitstatus_to_exitcode(status) == 0
    assert b'partial installation' in output and partial.read_text() == 'partial'
print('PASS: latest resolution/validation, version confirmation/decline/same/legacy/downgrade, mocked platforms, TTY, archive checks, cleanup, partial-state menu')
