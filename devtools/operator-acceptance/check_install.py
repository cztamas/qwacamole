"""Run in the disposable Ubuntu host against checksum-verified candidate files."""
from pathlib import Path
import json
import shutil
import subprocess
import tempfile

candidate=Path('/candidate')
package=candidate/'qwacamole-agent-0.1.0-alpha.1-linux-amd64'
results=[]
def rejected(name,args,error):
 p=subprocess.run(args,text=True,capture_output=True)
 assert p.returncode!=0 and error in p.stdout+p.stderr,(name,p.stdout,p.stderr)
 results.append({'check':name,'exit':p.returncode,'output':p.stdout,'error':p.stderr})
assert subprocess.run(['sh','-c','command -v go'],capture_output=True).returncode!=0
assert (Path('/opt/qwacamole-agent/bin/qwacamole-agent').stat().st_mode & 0o777)==0o755
rejected('existing installation prefix preserved',[str(package/'install.sh'),'--prefix','/opt/qwacamole-agent'],'Prefix already exists')
with tempfile.TemporaryDirectory(prefix='install-negative-') as directory:
 root=Path(directory)
 wrong=root/'qwacamole-agent-0.1.0-alpha.1-linux-arm64'
 shutil.copytree(package,wrong)
 prefix=root/'wrong-platform-install'
 rejected('wrong platform refused',[str(wrong/'install.sh'),'--prefix',str(prefix)],'Package does not match')
 assert not prefix.exists()
 shutil.copy(candidate/'SHA256SUMS',root/'SHA256SUMS')
 for path in candidate.iterdir():
  if path.is_file() and path.name!='SHA256SUMS':shutil.copy(path,root/path.name)
 archive=root/'qwacamole-agent-0.1.0-alpha.1-linux-amd64.tar.gz'
 with archive.open('ab') as file:file.write(b'corrupt')
 p=subprocess.run(['sha256sum','-c','SHA256SUMS'],cwd=root,text=True,capture_output=True)
 assert p.returncode!=0 and archive.name+': FAILED' in p.stdout,(p.stdout,p.stderr)
 results.append({'check':'corrupt archive checksum rejected before extraction/install','exit':p.returncode,'output':p.stdout,'error':p.stderr})
print(json.dumps({'no_go':True,'executable_mode':'0755','checks':results},indent=2))
