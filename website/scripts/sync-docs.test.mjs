import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveLink } from './sync-docs.mjs';
test('shared links work across nested routes and preserve fragments', () => {
  assert.equal(resolveLink('byo-credentials.md#inspect-and-import', 'docs/guides/caddy-plugin.md'), '/credentials/#inspect-and-import');
  assert.equal(resolveLink('../../README.md', 'docs/guides/demonstration.md'), 'https://github.com/cztamas/qwacamole/blob/main/README.md');
  assert.equal(resolveLink('../images/qualified-status.png', 'docs/guides/demonstration.md'), '/shared/docs/images/qualified-status.png');
  assert.equal(resolveLink('evidence-lifecycle-20261008.md#checks', 'docs/demos.md'), 'https://github.com/cztamas/qwacamole/blob/main/docs/evidence-lifecycle-20261008.md#checks');
  assert.equal(resolveLink('https://example.com', 'docs/demos.md'), 'https://example.com');
  assert.throws(() => resolveLink('../../secrets', 'README.md'), /escapes/);
});
