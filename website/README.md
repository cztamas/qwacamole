# Qwacamole website

Static Astro 7 + Starlight, with Node **24.21.0** (supported Node 24 LTS), npm
lockfile and pinned Wrangler. Use fnm/nvm with .node-version/.nvmrc. No Go,
browser, PKI, key or Cloudflare credential is needed for dev/check/build/preview.
The bundled npm 11 policy allows only the pinned esbuild/workerd binary setup
scripts; the optional fsevents installation hook is disabled.

```sh
cd website
npm ci
npm run dev           # http://127.0.0.1:4321
npm run check         # Astro/TypeScript + focused sharing/deployment tests
npm run build         # static dist/, search index, identity + internal link checks
npm run preview       # production output on loopback
```

Browser verification is a separate optional local prerequisite:

```sh
npx --no-install playwright install chromium
npm run test:browser
```

CI uses the same lockfile, checks/build and focused Chromium smoke at desktop/
mobile widths. It uses neither Go nor Cloudflare in its build/test job.

## Shared documentation

Edit canonical Markdown in ../docs/, README, SECURITY or CONTRIBUTING.
docs-map.mjs is the explicit page/asset publication allowlist. sync-docs parses
Markdown links/references/images, rewrites included docs to site routes, copies
only allowed public assets and links historical/source-only files to GitHub.
It generates ignored src/content/docs and public/shared; never edit those.
Frontmatter points Edit this page to the canonical source. Site navigation is
explicit in astro.config.mjs. Add a route and CI path filter together.

GitHub Markdown stays usable without Astro frontmatter or duplicate copies.
Historical reports remain dated in the repository; Firefox research is explicitly
dated. No recursive docs/state copying occurs. Generated keys/profiles/logs are
excluded. The homepage is a tailored Astro page; operator instructions are shared.

The dev server watches shared docs/assets and rebuilds generated content on edits.
All generated output, search caches, dependencies and browser artifacts are ignored.

## Deployment

See [the maintainer guide](../docs/website-deployment.md) for exact settings.
Deployment defaults disabled. Direct Upload in GitHub Actions is the sole
automatic mechanism, main-only and serialized, without PR previews. The separate
deployment job consumes checked dist/ and records/verifies full commit identity.
No live deployment or custom-domain binding is claimed yet.
