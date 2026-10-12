import { spawnSync } from 'node:child_process';
const result = spawnSync(process.execPath, ['node_modules/astro/bin/astro.mjs', ...process.argv.slice(2)], {
  stdio: 'inherit', env: { ...process.env, ASTRO_TELEMETRY_DISABLED: '1' },
});
if (result.error) throw result.error;
process.exit(result.status ?? 1);
