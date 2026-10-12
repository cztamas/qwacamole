"""Run only in the disposable root Ubuntu acceptance host after initial setup."""
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile

binary='/opt/qwacamole-agent/bin/qwacamole-agent'
args=[binary,'setup','--hostname','qwac.example.com','--upstream','http://127.0.0.1:8080','--chain','/inputs/qwac-chain.pem','--key','/inputs/qwac-leaf-key.pem','--authority','/inputs/qwac-root.pem','--email','operator@example.com']
results=[]
managed=list(json.loads(Path('/etc/qwacamole/setup-manifest.json').read_text()))+['/etc/qwacamole/setup-manifest.json']
state=Path('/var/lib/qwacamole/qwac.example.com')
(state/'acceptance-preservation-marker').write_text('owned acceptance state')
def snapshot():
 return {p:hashlib.sha256(Path(p).read_bytes()).hexdigest() for p in managed+[str(state/'acceptance-preservation-marker')]}
def run(name,argv=args,error=None):
 before=snapshot()
 p=subprocess.run(argv,text=True,capture_output=True)
 if error is None:
  assert p.returncode==0,(name,p.stdout,p.stderr)
 else:
  assert p.returncode!=0 and error in p.stderr,(name,p.returncode,p.stdout,p.stderr)
 assert snapshot()==before,name+' changed deployment/state'
 results.append({'check':name,'exit':p.returncode,'output':p.stdout,'error':p.stderr})
run('identical repeat preserves files and binding state')
for name,flag,value,error in [
 ('wrong hostname','--hostname','wrong.example.test','credential preflight'),
 ('mismatched key','--key','/inputs/tls-leaf-key.pem','credential preflight'),
 ('incomplete chain','--chain','/inputs/qwac-leaf.pem','credential preflight'),
 ('wrong authority','--authority','/inputs/tls-root.pem','credential preflight')]:
 changed=args.copy();changed[changed.index(flag)+1]=value;run(name,changed,error)
path=Path('/etc/qwacamole/agent.json');old=path.read_bytes()
try:
 path.write_bytes(old+b'\n');run('operator edit refused',error='existing file or credentials changed')
finally:path.write_bytes(old)
path=Path('/etc/qwacamole/key.pem')
try:
 path.chmod(0o644);run('public private-key permissions refused',error='incompatible type or permissions')
finally:path.chmod(0o600)
try:
 os.chown(path,65534,65534);run('unsafe ownership refused',error='unsafe ownership')
finally:os.chown(path,0,0)
certbot=Path('/usr/bin/certbot');saved=Path('/usr/bin/certbot.acceptance-backup')
try:
 certbot.rename(saved);run('missing Certbot refused',error='/usr/bin/certbot prerequisite')
finally:saved.rename(certbot)
# Fresh setup refuses listener/master conflicts without recreating any files.
with tempfile.TemporaryDirectory(prefix='operator-conflicts-') as directory:
 backup=Path(directory)
 for p in managed:
  destination=backup/p.lstrip('/');destination.parent.mkdir(parents=True,exist_ok=True);Path(p).rename(destination)
 state.rename(backup/'state')
 try:
  listener=socket.socket();listener.bind(('127.0.0.1',80));listener.listen()
  p=subprocess.run(args,text=True,capture_output=True);listener.close()
  assert p.returncode!=0 and 'listener 0.0.0.0:80 is unavailable' in p.stderr,(p.stdout,p.stderr)
  assert all(not Path(p).exists() for p in managed)
  results.append({'check':'occupied port rejected before writes','exit':p.returncode,'error':p.stderr})
  nginx=Path('/usr/sbin/nginx');old_nginx=Path('/usr/sbin/nginx.acceptance-backup')
  nginx.rename(old_nginx)
  try:
   nginx.write_text('#!/bin/sh\necho nginx-without-required-modules\n');nginx.chmod(0o755)
   p=subprocess.run(args,text=True,capture_output=True)
   assert p.returncode!=0 and 'Nginx needs --with-http_ssl_module' in p.stderr,(p.stdout,p.stderr)
   assert all(not Path(p).exists() for p in managed)
   results.append({'check':'missing Nginx capability rejected before writes','exit':p.returncode,'error':p.stderr})
  finally:nginx.unlink();old_nginx.rename(nginx)
 finally:
  for p in managed:(backup/p.lstrip('/')).rename(p)
  (backup/'state').rename(state)
print(json.dumps({'platform':subprocess.check_output(['uname','-sm'],text=True).strip(),'checks':results},indent=2))
