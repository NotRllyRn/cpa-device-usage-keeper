#!/usr/bin/env python3
"""Offline bootstrap tests: fake HTTPS, fake platform, real PTY and ZIP checks."""
import hashlib
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
case "$url" in *.sha256) cp "$FIXTURE/checksum" "$out" ;; *.zip) cp "$FIXTURE/archive.zip" "$out" ;; *) exit 1 ;; esac
''')
    env = dict(os.environ, PATH=str(tools)+':'+os.environ['PATH'], FIXTURE=td,
               DEVICE_USAGE_VERSION='v1.2.3', TEST_OS='Linux', TEST_ARCH='x86_64',
               MARKER=str(tmp/'invoked'), TMPDIR=td)
    def fixture(target, bad=None):
        asset = f'device-usage-v1.2.3-{target}.zip'
        with zipfile.ZipFile(tmp/'archive.zip', 'w') as z:
            for name in ['install.sh','install-macos.sh','install.ps1','LICENSE','BUILD.txt','SHA256SUMS','collector.tgz','device-sync-'+target]:
                body = 'fixture'
                if name == 'install.sh':
                    body = '#!/bin/bash\n[[ -t 0 ]] || exit 44\nprintf ok > "$MARKER"\n'
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
    def run(ok):
        (tmp/'invoked').unlink(missing_ok=True)
        pid, fd = pty.fork()
        if pid == 0:
            os.execvpe('bash', ['bash','-c', 'cat "$1/bootstrap.sh" | sh', '_', str(ROOT)], env)
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
        assert (tmp/'invoked').exists() == ok
        assert not list(tmp.glob('tmp.*')), 'Bootstrap leaked temporary directory'
    for system, arch, target in [('Linux','x86_64','linux-amd64'),('Darwin','arm64','darwin-arm64'),('Darwin','x86_64','darwin-amd64')]:
        env.update(TEST_OS=system, TEST_ARCH=arch)
        fixture(target); run(True)
        fixture(target, '../escape'); run(False)
        fixture(target, '/absolute'); run(False)
        fixture(target, 'symlink'); run(False)
        fixture(target); (tmp/'checksum').write_text('0'*64+'  '+f'device-usage-v1.2.3-{target}.zip\n'); run(False)
    env.update(TEST_OS='Linux', TEST_ARCH='aarch64'); run(False)
    env.update(TEST_ARCH='x86_64', DEVICE_USAGE_VERSION='latest'); run(False)
    env['DEVICE_USAGE_VERSION'] = 'v1.2.3\n../../other'; run(False)
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
print('PASS: mocked platforms/downloads, piped script with TTY, checksum/path rejection, cleanup, partial-state menu cancel')
