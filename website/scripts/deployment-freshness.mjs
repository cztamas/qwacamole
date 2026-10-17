const SHA = /^[a-f0-9]{40}$/;
const repository = 'cztamas/qwacamole';

export async function deploymentFreshness({ env, project, pagesUrl, request }) {
  if (!env.GITHUB_TOKEN || env.GITHUB_REPOSITORY !== repository)
    throw new Error('Freshness requires the repository contents-read token');
  async function compare(base, head) {
    const response = await request(`https://api.github.com/repos/${repository}/compare/${base}...${head}`, {
      headers: { Authorization: 'Bearer ' + env.GITHUB_TOKEN, Accept: 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28' },
      signal: AbortSignal.timeout(30000),
    });
    if (!response.ok) throw new Error('Source ancestry comparison unavailable; no upload attempted');
    const result = await response.json();
    if (!['ahead', 'behind', 'identical', 'diverged'].includes(result.status) || result.base_commit?.sha !== base || !SHA.test(result.merge_base_commit?.sha))
      throw new Error('Invalid source ancestry comparison; no upload attempted');
    return result;
  }
  // A later Go-only commit is allowed: require ancestry, not HEAD equality.
  const main = await request(`https://api.github.com/repos/${repository}/git/ref/heads/main`, {
    headers: { Authorization: 'Bearer ' + env.GITHUB_TOKEN, Accept: 'application/vnd.github+json' }, signal: AbortSignal.timeout(30000),
  });
  if (!main.ok) throw new Error('Main source unavailable; no upload attempted');
  const mainSha = (await main.json()).object?.sha;
  if (!SHA.test(mainSha)) throw new Error('Invalid main source; no upload attempted');
  const reachable = await compare(env.GITHUB_SHA, mainSha);
  if (!['ahead', 'identical'].includes(reachable.status) || reachable.merge_base_commit.sha !== env.GITHUB_SHA)
    throw new Error('Checked source is no longer on main; no upload attempted');

  // All retained successful production sources form the high-water mark, even
  // after a dashboard rollback. Completion timestamps are never source order.
  if (!Object.hasOwn(project, 'canonical_deployment'))
    throw new Error('Production deployment identity unavailable; no upload attempted');
  const deployments = [];
  if (project.canonical_deployment !== null) {
    if (project.canonical_deployment?.latest_stage?.status !== 'success')
      throw new Error('Canonical production source is not successful; no upload attempted');
    deployments.push(project.canonical_deployment);
  }
  for (let page = 1; ; page++) {
    // Pages rejects larger page sizes (including 100); 25 is accepted by the
    // live API. Keep traversing every page before deciding source freshness.
    const response = await request(pagesUrl + `/deployments?env=production&per_page=25&page=${page}`, {
      headers: { Authorization: 'Bearer ' + env.CLOUDFLARE_API_TOKEN }, signal: AbortSignal.timeout(30000),
    });
    if (!response.ok) throw new Error('Production history unavailable; no upload attempted');
    const body = await response.json();
    const info = body.result_info;
    if (!body.success || !Array.isArray(body.result) || info?.page !== page ||
        !Number.isInteger(info.total_pages) || info.total_pages < 0 || info.total_pages > 100 ||
        (info.total_pages === 0 && body.result.length))
      throw new Error('Incomplete production history; no upload attempted');
    deployments.push(...body.result);
    if (page >= info.total_pages) break;
  }
  const sources = new Set();
  for (const deployment of deployments) {
    if (deployment.environment !== 'production' || !deployment.latest_stage?.status)
      throw new Error('Invalid production history identity; no upload attempted');
    if (deployment.latest_stage.status !== 'success') {
      if (['idle', 'active'].includes(deployment.latest_stage.status))
        throw new Error('Another production write is unfinished; inspect Pages before uploading');
      if (!['failure', 'canceled'].includes(deployment.latest_stage.status))
        throw new Error('Unknown production stage; no upload attempted');
      continue;
    }
    const metadata = deployment.deployment_trigger?.metadata;
    if (deployment.latest_stage.name !== 'deploy' || metadata?.branch !== 'main' || !SHA.test(metadata?.commit_hash) || metadata.commit_dirty !== false)
      throw new Error('Successful production source identity unavailable; no upload attempted');
    sources.add(metadata.commit_hash);
  }
  let alreadyDeployed = false;
  for (const source of sources) {
    const comparison = await compare(source, env.GITHUB_SHA);
    if (comparison.status === 'behind')
      return { skipped: true, reason: `Superseded by production source ${source}; checked source ${env.GITHUB_SHA} is older.` };
    if (comparison.status === 'identical') {
      alreadyDeployed = true;
      continue;
    }
    if (comparison.status !== 'ahead' || comparison.merge_base_commit.sha !== source)
      throw new Error('Production and checked sources diverge; no upload attempted');
  }
  if (alreadyDeployed) return { skipped: true, reason: `Source ${env.GITHUB_SHA} already deployed; rerun performs no upload.` };
  return { skipped: false, reason: sources.size ? 'Checked source advances production history.' : 'First production deployment; no successful production source exists.' };
}
