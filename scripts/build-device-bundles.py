#!/usr/bin/env python3
"""Build private cross-platform client bundles; never installs or collects data."""
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import urllib.request
import re
import zipfile

root = Path(__file__).resolve().parent.parent
if len(sys.argv) != 3 or not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+', sys.argv[2]):
    raise SystemExit('Usage: build-device-bundles.py OUTPUT APPROVED_STABLE_TAG')
version = sys.argv[2]
out = Path(sys.argv[1]).resolve()
out.mkdir(mode=0o700, exist_ok=False)
artifacts = {
 ('darwin','arm64'): ('darwin-arm64','Rjg8t6QN6TQM7fYeTW2TxZ273Jh9+ct6F8IqWc9sdUZMrNur9QlHh9RHPlgBKsU+AQ9yapa/UeN3+naGT5LEBw=='),
 ('darwin','amd64'): ('darwin-x64','gnCOs5LMY/fDqumSZghQd3BuHCC2kqL4+pQehOs3jq1t4NuxL5DXga7vWpCQ4wXliBo3sHZyrd0TkTNEWkn2+A=='),
 ('windows','amd64'): ('win32-x64','DUUCQ/rdZedqW85ACZuC/Wamka4SRnrHInfmCjN5RWBzCBjFRJQyq8b3ZRJ42ct2f1a63s5IsLh0yQRdOWNVZg=='),
 ('windows','arm64'): ('win32-arm64','RmLT545qBHG7tRRLCi/Kj3wJeVsPVWNKxfo2guIfKXpIOaL3g0VPydbzipLJuGxICAFOfwYz3lTExJ54BNZ30g=='),
 ('linux','amd64'): ('linux-x64','C44a/Lay833Smjd2YV3SeqGCGCBjTXus81PHd949fOv6bbgw9xSUDdlJIzK1RT4MdlpOi8VmmnBydeMx9dlpng=='),
}
for (system, arch), (package, integrity) in artifacts.items():
    dest = out / f'{system}-{arch}'
    dest.mkdir(mode=0o700)
    binary = f'device-sync-{system}-{arch}' + ('.exe' if system == 'windows' else '')
    subprocess.run(['go','build','-trimpath','-o',str(dest / binary),'./cmd/device-sync'], cwd=root,
        env={**os.environ,'GOOS':system,'GOARCH':arch,'CGO_ENABLED':'0'}, check=True)
    url = f'https://registry.npmjs.org/@ccusage/ccusage-{package}/-/ccusage-{package}-20.0.43.tgz'
    with urllib.request.urlopen(url, timeout=60) as response:
        raw = response.read(33554433)
    if len(raw) > 33554432 or hashlib.sha512(raw).digest() != base64.b64decode(integrity):
        raise RuntimeError('collector integrity mismatch: '+package)
    (dest / 'collector.tgz').write_bytes(raw)
    for name in ['install.sh','install-macos.sh','install.ps1','LICENSE']:
        shutil.copyfile(root / name, dest / name)
    (dest / 'BUILD.txt').write_text('Source: '+subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip()+'\nTarget: '+system+'/'+arch+'\n')
    files = sorted(p for p in dest.iterdir() if p.is_file())
    (dest / 'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in files))
    archive = out / f'device-usage-{version}-{system}-{arch}.zip'
    with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED) as bundle:
        for path in sorted(dest.iterdir()):
            bundle.write(path, path.name)
    archive.with_suffix('.zip.sha256').write_text(hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name+'\n')
    print('Built', archive)
