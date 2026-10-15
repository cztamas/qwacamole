"""Run setup permission acceptance in fresh, disposable Ubuntu containers."""
import argparse
from pathlib import Path
import subprocess
import uuid

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--image', required=True)
parser.add_argument('--agent', required=True, type=Path)
parser.add_argument('--caddy', required=True, type=Path)
parser.add_argument('--fixtures', required=True, type=Path)
parser.add_argument('--output', required=True, type=Path)
args = parser.parse_args()
args.output.mkdir(parents=True, exist_ok=False)
helper = Path(__file__).with_name('check_setup_umask.py')

def docker(*arguments):
    return subprocess.run(['docker', *map(str, arguments)], check=True, text=True,
                          stdout=subprocess.PIPE).stdout.strip()

for mask in ('077', '0777'):
    name = 'qwacamole-setup-umask-' + uuid.uuid4().hex[:12]
    container = docker('run', '-d', '--platform', 'linux/amd64', '--name', name, args.image)
    try:
        docker('exec', container, 'mkdir', '-p', '/opt/qwacamole-agent/bin', '/opt/qwacamole-caddy/bin')
        docker('cp', args.agent.resolve(), container + ':/opt/qwacamole-agent/bin/qwacamole-agent')
        docker('cp', args.caddy.resolve(), container + ':/opt/qwacamole-caddy/bin/caddy-qwac')
        docker('cp', args.fixtures.resolve(), container + ':/inputs')
        docker('cp', helper.resolve(), container + ':/check_setup_umask.py')
        docker('exec', container, 'sh', '-c',
               'set -eu; chown -R root:root /opt/qwacamole-agent /opt/qwacamole-caddy /inputs; '
               'chmod 600 /inputs/*.pem; mv /var/lib/nginx /var/lib/nginx-original')
        for integration in ('agent', 'caddy'):
            result = docker('exec', container, 'python3', '/check_setup_umask.py', integration, mask)
            (args.output / (integration + '-' + mask + '.json')).write_text(result + '\n')
            print('PASS', integration, 'fresh/repeat setup, preservation and permissions, umask', mask)
    finally:
        docker('rm', '-f', container)
