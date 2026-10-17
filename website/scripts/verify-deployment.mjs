const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

export async function verifyDeployment({ url, sha, request = fetch, wait = pause }) {
  // A successful Pages upload can precede edge availability. Retry only this
  // unauthenticated GET: at most six 5-second requests and five 10-second waits.
  for (let attempt = 0; attempt < 6; attempt++) {
    let response;
    try {
      response = await request(url + 'build-info.json', { signal: AbortSignal.timeout(5000), cache: 'no-store' });
    } catch { /* A transient connection failure is also bounded below. */ }
    if (response?.ok) {
      if ((await response.json()).commit !== sha)
        throw new Error('Uploaded deployment source identity mismatch; inspect Pages before retrying');
      return;
    }
    if (response && ![403, 404, 429, 500, 502, 503, 504].includes(response.status)) break;
    if (attempt < 5) await wait(10000);
  }
  throw new Error('Uploaded deployment identity could not be verified; inspect Pages before retrying');
}
