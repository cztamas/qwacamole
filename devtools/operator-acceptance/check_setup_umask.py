"""Fresh disposable root Ubuntu 24.04 host only; run once per integration.

Usage: python3 check_setup_umask.py agent|caddy [octal-umask]
Requires binaries at /opt/qwacamole-{agent,caddy}/bin and fresh /inputs fixtures.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import urllib.request

integration = sys.argv[1]
assert integration in ('agent', 'caddy')
mask = int(sys.argv[2], 8) if len(sys.argv) > 2 else 0o077
binary = '/opt/qwacamole-agent/bin/qwacamole-agent' if integration == 'agent' else '/opt/qwacamole-caddy/bin/caddy-qwac'
command = [binary, 'setup' if integration == 'agent' else 'qwac-setup',
           '--hostname', 'qwac.example.com', '--upstream', 'http://127.0.0.1:8080',
           '--chain', '/inputs/qwac-chain.pem', '--key', '/inputs/qwac-leaf-key.pem',
           '--authority', '/inputs/qwac-root.pem', '--email', 'operator@example.com']

def run(expected=None):
    result = subprocess.run(command, text=True, capture_output=True,
                            preexec_fn=lambda: os.umask(mask))
    if expected is None:
        assert result.returncode == 0, (result.returncode, result.stdout, result.stderr)
    else:
        assert result.returncode != 0 and expected in result.stderr, (result.stdout, result.stderr)
    return result

run()
if integration == 'agent':
    manifest = json.loads(Path('/etc/qwacamole/setup-manifest.json').read_text())
    files = {path: 0o644 if path.endswith(('.service', '.timer')) else 0o600 for path in manifest}
    files['/etc/qwacamole/deploy-hook'] = 0o700
    files['/etc/qwacamole/setup-manifest.json'] = 0o600
    directories = {'/etc/qwacamole': 0o700, '/var/lib/qwacamole': 0o700,
                   '/var/lib/qwacamole/qwac.example.com': 0o700, '/run/qwacamole': 0o700,
                   '/var/lib/nginx/qwacamole': 0o755,
                   '/var/lib/nginx/qwacamole/client_body_temp': 0o700,
                   '/var/lib/qwacamole-acme': 0o755}
    directories['/var/lib/nginx'] = 0o755
    edited = Path('/etc/qwacamole/agent.json')
    unit = Path('/etc/systemd/system/qwacamole-agent.service')
    state = Path('/var/lib/qwacamole/qwac.example.com')
else:
    files = {str(path): 0o600 for path in Path('/etc/caddy-qwac').iterdir()}
    files['/etc/systemd/system/qwacamole-caddy.service'] = 0o644
    directories = {'/etc/caddy-qwac': 0o700, '/var/lib/caddy-qwac': 0o700}
    edited = Path('/etc/caddy-qwac/Caddyfile')
    unit = Path('/etc/systemd/system/qwacamole-caddy.service')
    state = Path('/var/lib/caddy-qwac')

for path, mode in files.items() | directories.items():
    actual = Path(path).stat().st_mode & 0o777
    assert actual == mode, (path, oct(actual), oct(mode))

marker = state / 'umask-preservation-marker'
marker.write_text('retain existing deployment state\n')
marker.chmod(0o600)
def snapshot():
    result = {}
    for path in list(files) + list(directories) + [str(marker)]:
        p = Path(path)
        st = p.stat()
        result[path] = {'mode': oct(st.st_mode & 0o777), 'uid': st.st_uid, 'gid': st.st_gid,
                        'mtime_ns': st.st_mtime_ns,
                        'sha256': hashlib.sha256(p.read_bytes()).hexdigest() if p.is_file() else None}
    return result

before = snapshot()
run()
assert snapshot() == before, 'repeat modified existing files, modes, timestamps or state'
original = edited.read_bytes()
try:
    edited.write_bytes(original + b'\n')
    before = snapshot()
    run('existing file or credentials changed')
    assert snapshot() == before, 'setup changed operator-edited data'
finally:
    edited.write_bytes(original)
try:
    unit.chmod(0o600)
    before = snapshot()
    run('incompatible' if integration == 'agent' else 'existing file or credentials changed')
    assert snapshot() == before, 'setup silently repaired an existing operator mode'
finally:
    unit.chmod(0o644)

worker_access = None
if integration == 'agent':
    token = Path('/var/lib/qwacamole-acme/.well-known/acme-challenge/umask-regression')
    token.parent.mkdir(parents=True)
    token.parent.parent.chmod(0o755)
    token.parent.chmod(0o755)
    token.write_text('readable-by-www-data\n')
    token.chmod(0o644)
    args = ['/usr/sbin/nginx', '-p', '/var/lib/nginx/qwacamole', '-c', '/etc/qwacamole/nginx.conf']
    subprocess.run(args, check=True, capture_output=True)
    try:
        for attempt in range(50):
            try:
                request = urllib.request.Request('http://127.0.0.1/.well-known/acme-challenge/umask-regression',
                                                 headers={'Host': 'qwac.example.com'})
                with urllib.request.urlopen(request, timeout=1) as response:
                    assert response.status == 200 and response.read() == b'readable-by-www-data\n'
                worker_access = 'HTTP 200 through generated Nginx with www-data workers'
                break
            except OSError:
                if attempt == 49: raise
                time.sleep(.1)
    finally:
        subprocess.run(args + ['-s', 'quit'], check=True, capture_output=True)

print(json.dumps({'integration': integration, 'umask': oct(mask), 'fresh_modes': 'pass',
                  'repeat_preserves_files_and_state': 'pass', 'operator_edits_preserved': 'pass',
                  'operator_modes_preserved': 'pass', 'worker_challenge_access': worker_access,
                  'required_modes': {p: oct(m) for p, m in files.items() | directories.items()}}, indent=2))
