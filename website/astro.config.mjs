import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import path from 'node:path';
import { root, syncDocs } from './scripts/sync-docs.mjs';
import { documents, assets } from './docs-map.mjs';
import caddyGrammar from './caddy-grammar.mjs';
import { visit } from 'unist-util-visit';
export default defineConfig({
  site: 'https://qwacamole.com',
  output: 'static',
  cacheDir: './.astro/cache',
  trailingSlash: 'always',
  vite: { server: { strictPort: true }, plugins: [{
    name: 'shared-docs-watch',
    configureServer(server) {
      const inputs = [...Object.keys(documents), ...assets].map(p => path.join(root, p));
      server.watcher.add(inputs);
      let pending = Promise.resolve();
      server.watcher.on('change', file => {
        if (inputs.includes(file)) {
          pending = pending.then(syncDocs)
            .then(() => server.ws.send({ type: 'full-reload' }))
            .catch(error => server.config.logger.error(error.message));
        }
      });
    },
  }] },
  integrations: [starlight({
    title: 'qwacamole',
    disable404Route: true,
    expressiveCode: {
      shiki: { langs: [caddyGrammar] },
      plugins: [{ name: 'keyboard-code-scrolling', hooks: {
        postprocessRenderedBlock({ renderData, codeBlock }) {
          visit(renderData.blockAst, 'element', node => {
            if (node.tagName === 'pre') {
              node.properties.tabIndex = 0;
              node.properties['aria-label'] = (codeBlock.language || 'Text') + ' code sample';
            }
          });
        },
      } }],
    },
    description: 'Supplied 2-QWAC credentials. Bound to the TLS you serve.',
    logo: { src: './src/assets/mark.svg' },
    favicon: '/favicon.svg',
    customCss: ['./src/styles/docs.css'],
    social: [{ icon: 'github', label: 'Source (private repository)', href: 'https://github.com/cztamas/qwacamole' }],
    editLink: { baseUrl: 'https://github.com/cztamas/qwacamole/edit/main/' },
    sidebar: [
      { label: 'Start here', items: [{ label: 'Overview', slug: 'overview' }, { label: 'Caddy quickstart', slug: 'caddy/quickstart' }, { label: 'Certbot / Nginx quickstart', slug: 'nginx/quickstart' }] },
      { label: 'Configure', items: [{ label: 'Caddy reference', slug: 'caddy/reference' }, { label: 'Agent reference', slug: 'nginx/reference' }, { label: 'Keys, CSRs & credentials', slug: 'credentials' }] },
      { label: 'Operate', items: [{ label: 'Discovery & maintenance', slug: 'operations' }, { label: 'Agent migration & rollback', slug: 'nginx/migration' }, { label: 'Support & troubleshooting', slug: 'support' }] },
      { label: 'Evaluate & support', items: [{ label: 'Isolated demonstration', slug: 'demonstration' }, { label: 'License & attribution', slug: 'project/licensing' }, { label: 'Security reporting', slug: 'project/security' }] },
    ],
  })],
});
