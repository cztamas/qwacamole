import { execFileSync } from 'node:child_process';
import { writeFile } from 'node:fs/promises';
const commit = process.env.GITHUB_SHA || execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
if (!/^[a-f0-9]{40}$/.test(commit)) throw new Error('Expected full source commit');
await writeFile('dist/build-info.json', JSON.stringify({ commit, site: 'https://qwacamole.com', deployment: 'static' }, null, 2) + '\n');
