import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { deploy } from './deploy.mjs';

const env = { GITHUB_REF: 'refs/heads/main', GITHUB_EVENT_NAME: 'push', GITHUB_SHA: 'b'.repeat(40),
  GITHUB_REPOSITORY: 'cztamas/qwacamole', GITHUB_TOKEN: 'synthetic-github',
  WEBSITE_DEPLOY_ENABLED: 'true', CLOUDFLARE_ACCOUNT_ID: 'a'.repeat(32), CLOUDFLARE_PAGES_PROJECT: 'qwacamole', CLOUDFLARE_API_TOKEN: 'synthetic' };
function fixture(t, changes = {}, projectChanges = {}) {
  const directory = mkdtempSync(path.join(tmpdir(), 'site-test-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  writeFileSync(path.join(directory, 'build-info.json'), JSON.stringify({ commit: env.GITHUB_SHA }));
  const record = { type: 'pages-deploy-detailed', pages_project: 'qwacamole', deployment_id: 'synthetic-id',
    url: 'https://123abc.qwacamole.pages.dev/', environment: 'production', production_branch: 'main',
    deployment_trigger: { metadata: { commit_hash: env.GITHUB_SHA } }, ...changes };
  let writes = 0;
  return {
    options: { env, directory,
      execute: (command, args, options) => {
        writes++;
        assert.equal(command, './node_modules/.bin/wrangler');
        assert.equal(args[args.indexOf('--branch') + 1], 'main');
        assert.equal(args[args.indexOf('--commit-hash') + 1], env.GITHUB_SHA);
        assert.equal(args[args.indexOf('--project-name') + 1], 'qwacamole');
        writeFileSync(options.env.WRANGLER_OUTPUT_FILE_PATH, JSON.stringify(record) + '\n');
      },
      request: async (url, options) => {
        if (url.startsWith('https://api.github.com/')) {
          assert.equal(options.headers.Authorization, 'Bearer synthetic-github');
          if (url.endsWith('/git/ref/heads/main')) return { ok: true, json: async () => ({ object: { sha: env.GITHUB_SHA } }) };
          return { ok: true, json: async () => ({ status: 'identical', base_commit: { sha: env.GITHUB_SHA }, merge_base_commit: { sha: env.GITHUB_SHA } }) };
        }
        if (url.startsWith('https://api.cloudflare.com/')) {
          assert.equal(options.headers.Authorization, 'Bearer synthetic');
          if (url.includes('/deployments?')) return { ok: true, json: async () => ({ success: true, result: [], result_info: { page: 1, total_pages: 0 } }) };
          return { ok: true, json: async () => ({ success: true, result: { name: 'qwacamole', production_branch: 'main', canonical_deployment: null, ...projectChanges } }) };
        }
        assert.equal(url, record.url + 'build-info.json');
        return { ok: true, json: async () => ({ commit: env.GITHUB_SHA }) };
      } },
    writes: () => writes,
  };
}
test('checked artifact uploads once with main/source identity and verified static result', async t => {
  const f = fixture(t);
  assert.deepEqual(await deploy(f.options), { sha: env.GITHUB_SHA, url: 'https://123abc.qwacamole.pages.dev', deploymentId: 'synthetic-id' });
  assert.equal(f.writes(), 1);
});

function production(sha) {
  return { environment: 'production', latest_stage: { name: 'deploy', status: 'success' },
    deployment_trigger: { metadata: { branch: 'main', commit_hash: sha, commit_dirty: false } } };
}
function ordering(t) {
  const f = fixture(t);
  // Hash lexical order is intentionally opposite ancestry. A < B < Go-only C.
  const A = 'f'.repeat(40), B = env.GITHUB_SHA, C = 'a'.repeat(40);
  const order = [A, B, C];
  let history = [], current = null;
  f.options.request = async url => {
    if (url.includes('/git/ref/heads/main')) return { ok: true, json: async () => ({ object: { sha: C } }) };
    if (url.includes('/compare/')) {
      const [base, head] = url.split('/compare/')[1].split('...');
      const delta = order.indexOf(head) - order.indexOf(base);
      return { ok: true, json: async () => ({ status: delta > 0 ? 'ahead' : delta < 0 ? 'behind' : 'identical',
        base_commit: { sha: base }, merge_base_commit: { sha: delta >= 0 ? base : head } }) };
    }
    if (url.includes('/deployments?')) return { ok: true, json: async () => ({ success: true, result: history, result_info: { page: 1, total_pages: 1 } }) };
    if (url.startsWith('https://api.cloudflare.com/')) return { ok: true, json: async () => ({ success: true, result: { name: 'qwacamole', production_branch: 'main', canonical_deployment: current } }) };
    return { ok: true, json: async () => ({ commit: f.options.env.GITHUB_SHA }) };
  };
  return { f, A, B, setSource(sha, event = 'push') {
    f.options.env = { ...env, GITHUB_SHA: sha, GITHUB_EVENT_NAME: event };
    writeFileSync(path.join(f.options.directory, 'build-info.json'), JSON.stringify({ commit: sha }));
  }, deployed(sha) { current = production(sha); history.push(current); }, rollback(sha) { current = production(sha); } };
}
test('A starts, B deploys while A checks; delayed A and superseded rerun/manual dispatch upload nothing', async t => {
  const o = ordering(t);
  o.setSource(o.A); // A's checked artifact starts first, but finishes later.
  o.setSource(o.B);
  assert.equal((await deploy(o.f.options)).sha, o.B); // Go-only main C does not suppress B.
  o.deployed(o.B);
  for (const event of ['push', 'push', 'workflow_dispatch']) {
    o.setSource(o.A, event);
    const result = await deploy(o.f.options);
    assert.equal(result.skipped, true);
    assert.match(result.reason, /Superseded/);
    assert.equal(o.f.writes(), 1); // Only B ever executes Wrangler.
  }
  o.setSource(o.B, 'workflow_dispatch');
  assert.equal((await deploy(o.f.options)).skipped, true);
  assert.equal(o.f.writes(), 1);
});
test('rollback retains the production high-water mark and later history pages are read', async t => {
  const o = ordering(t);
  o.deployed(o.B); o.rollback(o.A); o.setSource(o.A);
  const read = o.f.options.request;
  o.f.options.request = async url => url.includes('/deployments?') ? ({ ok: true, json: async () => ({ success: true,
    result: url.endsWith('page=1') ? [] : [production(o.B)], result_info: { page: url.endsWith('page=1') ? 1 : 2, total_pages: 2 } }) }) : read(url);
  assert.equal((await deploy(o.f.options)).skipped, true);
  assert.equal(o.f.writes(), 0);
});
test('unknown production identity, diverged ancestry, API failure and unfinished writes never upload', async t => {
  for (const change of ['missing-canonical', 'unknown-sha', 'diverged', 'failed-main', 'failed-compare', 'failed-history', 'bad-pagination', 'unfinished']) {
    const o = ordering(t); o.deployed(o.A); o.setSource(o.B);
    const read = o.f.options.request;
    o.f.options.request = async url => {
      if ((change === 'failed-main' && url.includes('/git/ref/')) || (change === 'failed-compare' && url.includes('/compare/')) ||
          (change === 'failed-history' && url.includes('/deployments?'))) return { ok: false, status: 403 };
      const response = await read(url), body = await response.json();
      if (url.includes('/compare/') && change === 'diverged') body.status = 'diverged';
      if (url.includes('/deployments?') && change === 'bad-pagination') delete body.result_info;
      if (url.startsWith('https://api.cloudflare.com/') && !url.includes('/deployments?')) {
        if (change === 'missing-canonical') delete body.result.canonical_deployment;
        if (change === 'unknown-sha') body.result.canonical_deployment.deployment_trigger.metadata.commit_hash = '';
        if (change === 'unfinished') body.result.canonical_deployment.latest_stage.status = 'active';
      }
      return { ok: true, json: async () => body };
    };
    await assert.rejects(deploy(o.f.options), /no upload|unfinished/);
    assert.equal(o.f.writes(), 0, change);
  }
});
test('wrong checked source stops before upload', async t => {
  const f = fixture(t);
  writeFileSync(path.join(f.options.directory, 'build-info.json'), JSON.stringify({ commit: 'c'.repeat(40) }));
  await assert.rejects(deploy(f.options), /artifact\/source/);
  assert.equal(f.writes(), 0);
});
test('wrong production metadata and failed post-upload identity fail without retry', async t => {
  const f = fixture(t, { environment: 'preview' });
  await assert.rejects(deploy(f.options), /production identity/);
  assert.equal(f.writes(), 1);
  const g = fixture(t);
  const originalRequest = g.options.request;
  g.options.request = async (url, options) => url.startsWith('https://api.cloudflare.com/') || url.startsWith('https://api.github.com/')
    ? originalRequest(url, options) : ({ ok: true, json: async () => ({ commit: 'c'.repeat(40) }) });
  await assert.rejects(deploy(g.options), /could not be verified/);
  assert.equal(g.writes(), 1);
});
test('project branch, duplicate Git builds and API denial stop before upload', async t => {
  for (const project of [
    { production_branch: 'master' },
    { source: { type: 'github', config: { production_deployments_enabled: true, preview_deployment_setting: 'all' } } },
  ]) {
    const f = fixture(t, {}, project);
    await assert.rejects(deploy(f.options), /no upload attempted/);
    assert.equal(f.writes(), 0);
  }
  const f = fixture(t);
  f.options.request = async () => ({ ok: false, status: 403 });
  await assert.rejects(deploy(f.options), /preflight failed/);
  assert.equal(f.writes(), 0);
});
test('upload failure does not expose captured output or retry', async t => {
  const f = fixture(t);
  let calls = 0;
  f.options.execute = () => { calls++; throw Object.assign(new Error('synthetic private CLI output'), { status: 1 }); };
  await assert.rejects(deploy(f.options), error => error.message.includes('upload failed') && !error.message.includes('private'));
  assert.equal(calls, 1);
});
