import { execFileSync } from 'node:child_process';
import { appendFileSync, readFileSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { deploymentSetup } from './deployment-setup.mjs';
import { deploymentFreshness } from './deployment-freshness.mjs';

export async function deploy({ env = process.env, directory = 'dist', execute = execFileSync, request = fetch } = {}) {
  if (!deploymentSetup(env)) throw new Error('Deployment is disabled');
  const sha = env.GITHUB_SHA;
  const identity = JSON.parse(readFileSync(path.join(directory, 'build-info.json'), 'utf8'));
  if (!/^[a-f0-9]{40}$/.test(sha) || identity.commit !== sha) throw new Error('Checked artifact/source identity mismatch');
  const projectResponse = await request(
    'https://api.cloudflare.com/client/v4/accounts/' + env.CLOUDFLARE_ACCOUNT_ID + '/pages/projects/' + env.CLOUDFLARE_PAGES_PROJECT,
    { headers: { Authorization: 'Bearer ' + env.CLOUDFLARE_API_TOKEN }, signal: AbortSignal.timeout(30000) },
  );
  if (!projectResponse.ok) throw new Error('Pages project preflight failed; no upload attempted');
  const project = await projectResponse.json();
  if (!project.success || project.result?.name !== env.CLOUDFLARE_PAGES_PROJECT || project.result.production_branch !== 'main')
    throw new Error('Pages project must exist with main as production branch; no upload attempted');
  const source = project.result.source;
  if (source && ['github', 'gitlab'].includes(source.type) &&
      source.config?.deployments_enabled !== false &&
      (source.config?.production_deployments_enabled !== false || source.config?.preview_deployment_setting !== 'none'))
    throw new Error('Disable automatic Git production and preview builds before Direct Upload; no upload attempted');
  const freshness = await deploymentFreshness({ env, project: project.result, request,
    pagesUrl: 'https://api.cloudflare.com/client/v4/accounts/' + env.CLOUDFLARE_ACCOUNT_ID + '/pages/projects/' + env.CLOUDFLARE_PAGES_PROJECT });
  if (freshness.skipped) return { ...freshness, sha };
  const scratch = mkdtempSync(path.join(tmpdir(), 'qwacamole-pages-'));
  const outputFile = path.join(scratch, 'output.jsonl');
  try {
    try {
      const { GITHUB_TOKEN: _githubReadToken, ...uploadEnv } = env;
      execute('./node_modules/.bin/wrangler', [
        'pages', 'deploy', directory, '--project-name', env.CLOUDFLARE_PAGES_PROJECT,
        '--branch', 'main', '--commit-hash', sha, '--commit-message', 'Documentation at ' + sha,
        '--commit-dirty=false',
      ], { stdio: ['ignore', 'pipe', 'pipe'], env: { ...uploadEnv, WRANGLER_OUTPUT_FILE_PATH: outputFile, WRANGLER_SEND_METRICS: 'false' } });
    } catch (error) {
      // Do not print captured process output or credentials; never retry a write.
      throw new Error('Pages upload failed (exit ' + (error.status ?? 'unknown') + '). Inspect the project before retrying.');
    }
    const records = readFileSync(outputFile, 'utf8').trim().split('\n').map(line => JSON.parse(line));
    const deployments = records.filter(record => record.type === 'pages-deploy-detailed');
    if (deployments.length !== 1) throw new Error('Uncertain upload record; inspect Pages before retrying');
    const record = deployments[0];
    const url = new URL(record.url);
    if (record.pages_project !== env.CLOUDFLARE_PAGES_PROJECT ||
        record.environment !== 'production' || record.production_branch !== 'main' ||
        record.deployment_trigger?.metadata?.commit_hash !== sha ||
        !record.deployment_id || url.protocol !== 'https:' || url.pathname !== '/' || url.search || url.hash ||
        !url.hostname.endsWith('.' + env.CLOUDFLARE_PAGES_PROJECT + '.pages.dev'))
      throw new Error('Upload source/project/production identity mismatch; inspect Pages before retrying');
    const response = await request(url.href + 'build-info.json', { signal: AbortSignal.timeout(30000), cache: 'no-store' });
    if (!response.ok || (await response.json()).commit !== sha)
      throw new Error('Uploaded deployment identity could not be verified; inspect Pages before retrying');
    return { sha, url: url.origin, deploymentId: record.deployment_id };
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const result = await deploy();
  if (result.skipped) {
    appendFileSync(process.env.GITHUB_STEP_SUMMARY, '## Website deployment skipped\n' + result.reason + '\n\nNo production upload performed.\n');
    appendFileSync(process.env.GITHUB_OUTPUT, 'deployment=skipped\n');
    console.log('SKIPPED: ' + result.reason);
  } else {
    appendFileSync(process.env.GITHUB_STEP_SUMMARY, '## Website deployed\nCommit: `' + result.sha + '`\n\nDeployment: ' + result.url + '\n\nDeployment ID: `' + result.deploymentId + '`\n\nVerified production metadata and build-info.json at the immutable deployment URL.\n');
    console.log('Verified website commit ' + result.sha + ' at ' + result.url);
  }
}
