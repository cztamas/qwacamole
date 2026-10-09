import test from 'node:test';
import assert from 'node:assert/strict';
import { copyFileSync, mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { execFileSync, spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const source = fileURLToPath(new URL('../../scripts/start-website.sh', import.meta.url));
function fixture(t, withNode = false) {
  const root = mkdtempSync(path.join(tmpdir(), 'website setup '));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const dir of ['scripts', 'website', 'bin']) mkdirSync(path.join(root, dir));
  copyFileSync(source, path.join(root, 'scripts/start-website.sh'));
  writeFileSync(path.join(root, 'website/.node-version'), '24.21.0\n');
  for (const binary of ['bash', 'dirname']) {
    const executable = execFileSync('which', [binary], { encoding: 'utf8' }).trim();
    symlinkSync(executable, path.join(root, 'bin', binary));
  }
  if (withNode) {
    symlinkSync(process.execPath, path.join(root, 'bin/node'));
    symlinkSync(path.join(path.dirname(process.execPath), 'npm'), path.join(root, 'bin/npm'));
  }
  return args => spawnSync(path.join(root, 'scripts/start-website.sh'), args, {
    cwd: tmpdir(), env: { PATH: path.join(root, 'bin') }, encoding: 'utf8',
  });
}
test('missing Node gives the exact pinned setup commands from another directory', t => {
  const result = fixture(t)([]);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /Node 24\.21\.0 is required but node is not on PATH/);
  assert.match(result.stderr, /fnm install 24\.21\.0\n  fnm use 24\.21\.0/);
  assert.match(result.stderr, /npm --prefix .*website ci/);
});
test('clean checkout without dependencies fails clearly with an npm ci command', t => {
  const result = fixture(t, true)(['--port', '4400']);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /Website dependencies are missing or incomplete/);
  assert.match(result.stderr, /npm --prefix .*website ci/);
});
test('invalid ports and missing port arguments fail before startup', t => {
  const run = fixture(t);
  for (const port of ['0', '65536', 'abc', '99999999999999999999']) {
    const result = run(['--port', port]);
    assert.equal(result.status, 2);
    assert.match(result.stderr, /port must be an integer from 1 to 65535/);
  }
  assert.equal(run(['--port']).status, 2);
  assert.equal(run(['--help']).status, 0);
});
