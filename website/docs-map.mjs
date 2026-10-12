// This allowlist is the entire publication boundary. Never copy docs/ recursively.
export const documents = {
  'docs/guides/site-overview.md': 'overview',
  'docs/guides/binary-installation.md': 'installation',
  'docs/guides/source-installation.md': 'installation/source',
  'docs/guides/caddy-quickstart.md': 'caddy/quickstart',
  'docs/guides/nginx-quickstart.md': 'nginx/quickstart',
  'docs/guides/caddy-plugin.md': 'caddy/reference',
  'docs/guides/agent-nginx.md': 'nginx/reference',
  'docs/guides/byo-credentials.md': 'credentials',
  'docs/guides/operations.md': 'operations',
  'docs/guides/agent-migration.md': 'nginx/migration',
  'docs/guides/site-support.md': 'support',
  'docs/guides/demonstration.md': 'demonstration',
  'docs/guides/licensing.md': 'project/licensing',
  'SECURITY.md': 'project/security',
};
export const assets = [
  'docs/images/qualified-status.png',
  'docs/images/qwac-certificate-viewer.png',
  'docs/images/firefox-qualified-status.png',
  'config/Caddyfile.minimal',
  'config/Caddyfile.reverse-proxy',
  'config/Caddyfile.file-server',
  'config/Caddyfile.pebble',
  'config/plugin.json.example',
  'config/agent.json.example',
  'LICENSE', 'NOTICE',
];
