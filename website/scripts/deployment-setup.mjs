import { appendFileSync } from 'node:fs';
export function deploymentSetup(env) {
  if (env.GITHUB_REF !== 'refs/heads/main' || !['push', 'workflow_dispatch'].includes(env.GITHUB_EVENT_NAME))
    throw new Error('Production deployment requires a main push or main manual dispatch');
  const enabled = env.WEBSITE_DEPLOY_ENABLED || 'false';
  if (!['true', 'false'].includes(enabled)) throw new Error('WEBSITE_DEPLOY_ENABLED must be true or false');
  if (enabled === 'false') return false;
  for (const key of ['CLOUDFLARE_ACCOUNT_ID', 'CLOUDFLARE_PAGES_PROJECT', 'CLOUDFLARE_API_TOKEN'])
    if (!env[key]?.trim()) throw new Error('Enabled deployment missing ' + key);
  if (!/^[a-f0-9]{32}$/.test(env.CLOUDFLARE_ACCOUNT_ID)) throw new Error('Invalid account ID');
  if (!/^[a-z0-9][a-z0-9-]*$/.test(env.CLOUDFLARE_PAGES_PROJECT)) throw new Error('Invalid Pages project name');
  return true;
}
if (process.argv[1]?.endsWith('/deployment-setup.mjs')) {
  const enabled = deploymentSetup(process.env);
  appendFileSync(process.env.GITHUB_OUTPUT, 'enabled=' + enabled + '\n');
  if (!enabled) appendFileSync(process.env.GITHUB_STEP_SUMMARY,
    '## Website deployment disabled\nNo upload performed. Configure the Direct Upload project (main production branch), CLOUDFLARE_ACCOUNT_ID/CLOUDFLARE_PAGES_PROJECT variables, CLOUDFLARE_API_TOKEN secret and set WEBSITE_DEPLOY_ENABLED=true. See docs/website-deployment.md.\n');
}
