import test from 'node:test';
import assert from 'node:assert/strict';
import { verifyDeployment } from './verify-deployment.mjs';
const url = 'https://123abc.qwacamole.pages.dev/';
const sha = 'b'.repeat(40);

test('new deployment tolerates edge propagation without uploading or sending credentials', async () => {
  const statuses = [403, 404, 503, 200], waits = [];
  await verifyDeployment({ url, sha, wait: async ms => waits.push(ms), request: async (target, options) => {
    assert.equal(target, url + 'build-info.json');
    assert.equal(options.headers, undefined);
    assert.equal(options.method, undefined); // fetch defaults to GET.
    const status = statuses.shift();
    return { ok: status === 200, status, json: async () => ({ commit: sha }) };
  } });
  assert.deepEqual(statuses, []);
  assert.deepEqual(waits, [10000, 10000, 10000]);
});

test('persistent denial and connection failure stop at the bounded read budget', async () => {
  for (const failure of ['http', 'network']) {
    let reads = 0, waits = 0;
    await assert.rejects(verifyDeployment({ url, sha, wait: async () => { waits++; }, request: async () => {
      reads++;
      if (failure === 'network') throw new Error('synthetic private connection details');
      return { ok: false, status: 403 };
    } }), /identity could not be verified/);
    assert.equal(reads, 6);
    assert.equal(waits, 5);
  }
});

test('wrong source and permanent client errors fail immediately', async () => {
  for (const response of [{ ok: true, status: 200, json: async () => ({ commit: 'c'.repeat(40) }) }, { ok: false, status: 400 }]) {
    let reads = 0;
    await assert.rejects(verifyDeployment({ url, sha, wait: async () => assert.fail('must not wait'), request: async () => {
      reads++; return response;
    } }), /identity/);
    assert.equal(reads, 1);
  }
});
