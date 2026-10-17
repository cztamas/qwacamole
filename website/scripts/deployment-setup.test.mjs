import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { deploymentSetup } from './deployment-setup.mjs';
import { documents, assets } from '../docs-map.mjs';
const valid = { GITHUB_REF: 'refs/heads/main', GITHUB_EVENT_NAME: 'push', WEBSITE_DEPLOY_ENABLED: 'true', CLOUDFLARE_ACCOUNT_ID: 'a'.repeat(32), CLOUDFLARE_PAGES_PROJECT: 'qwacamole', CLOUDFLARE_API_TOKEN: 'synthetic' };
test('default disabled, enabled requires every setting and main-only event', () => {
  assert.equal(deploymentSetup({ GITHUB_REF: valid.GITHUB_REF, GITHUB_EVENT_NAME: 'push' }), false);
  assert.equal(deploymentSetup(valid), true);
  assert.equal(deploymentSetup({ ...valid, GITHUB_EVENT_NAME: 'workflow_dispatch' }), true);
  for (const key of ['CLOUDFLARE_ACCOUNT_ID', 'CLOUDFLARE_PAGES_PROJECT', 'CLOUDFLARE_API_TOKEN']) assert.throws(() => deploymentSetup({ ...valid, [key]: '' }), /missing/);
  assert.throws(() => deploymentSetup({ ...valid, GITHUB_REF: 'refs/heads/feature' }), /main/);
  assert.throws(() => deploymentSetup({ ...valid, GITHUB_EVENT_NAME: 'pull_request' }), /main/);
  assert.throws(() => deploymentSetup({ ...valid, WEBSITE_DEPLOY_ENABLED: 'tru' }), /true or false/);
});
test('workflow watches every shared build input for both push and PR', () => {
  const workflow = readFileSync('../.github/workflows/website.yml', 'utf8');
  for (const input of [...Object.keys(documents), ...assets, 'scripts/start-website.sh']) {
    assert.equal(workflow.split("'" + input + "'").length - 1, 2, input + ' must be watched for PR and main push');
  }
});
test('mistakenly pasted commands, quotes and whitespace are rejected without printing the secret', () => {
  for (const token of ['Bearer synthetic-private', ' synthetic-private', 'synthetic-private\n', '"synthetic-private"', "'synthetic-private'", 'curl -H synthetic-private']) {
    assert.throws(() => deploymentSetup({ ...valid, CLOUDFLARE_API_TOKEN: token }), error => {
      assert.match(error.message, /only the token value/);
      assert.doesNotMatch(error.message, /synthetic-private/);
      return true;
    });
  }
});
