# Qwacamole website

Static Astro 7 + Starlight, with Node **24.21.0** (supported Node 24 LTS), npm
lockfile and pinned Wrangler. No Go,
browser, PKI, key or Cloudflare credential is needed for dev/check/build/preview.
The bundled npm 11 policy allows only the pinned esbuild/workerd binary setup
scripts; the optional fsevents installation hook is disabled.

## Install and start locally

From the repository root, using your installed fnm Node version manager:

```sh
fnm install 24.21.0
fnm use 24.21.0
npm --prefix website ci
./scripts/start-website.sh
```

If you use nvm instead, the equivalent Node setup is `nvm install 24.21.0` and
`nvm use 24.21.0`. The version files in website/ pin the same release. Node's
distribution supplies npm; no global Astro install is needed.

Open **http://127.0.0.1:4321/**. The server stays in the foreground. **Ctrl+C**
stops it and releases the port. Editing website files or canonical shared
Markdown/assets triggers live reload. After fixing an invalid Markdown save,
the watcher recovers without restarting. Dependencies are installed separately;
the script prints exact setup commands and exits if Node/dependencies are missing.

Choose another localhost port with either form (the flag takes precedence):

```sh
./scripts/start-website.sh --port 4400
PORT=4400 ./scripts/start-website.sh
```

Paths resolve relative to the script, so an absolute invocation such as
`/path/to/qwacamole/scripts/start-website.sh --port 4400` works from another
directory. An occupied port fails rather than silently selecting a different
URL. The script needs no Go services, Cloudflare credentials or deployment setup.

## Check and preview the production build

From the repository root:

```sh
npm --prefix website run check
npm --prefix website run build
npm --prefix website run preview -- --ignore-lock
```

Browse http://127.0.0.1:4321/ and stop with **Ctrl+C**. Stop the dev server first,
or use `npm --prefix website run preview -- --port 4400 --ignore-lock`.
Preview serves the static build; rebuild to include edits. Pagefind search works
in production preview; Starlight's dev server explains that search requires a
built site. Review responsive layout/live edits with the start script and verify
search with the production preview. Checks/build/preview also require no Go or
Cloudflare configuration.

Browser verification is a separate optional local prerequisite:

```sh
cd website
npx --no-install playwright install chromium
npm run test:browser
```

CI uses the same lockfile, checks/build and focused Chromium smoke at desktop/
mobile widths. It uses neither Go nor Cloudflare in its build/test job.
The build finalizes Pagefind's built-in index through its
[supported getFiles API](https://pagefind.app/docs/node-api/)
with awaited asset writes and nonempty checks, avoiding incomplete native output
observed on macOS. Search stays fully static; no retry or service is added.

## Shared documentation

Edit canonical product Markdown in ../docs/guides/ or the root SECURITY.md.
Root README, CONTRIBUTING, engineering reports, Firefox research and maintainer
release/website guides are repository-only. See ../docs/README.md for audiences.
docs-map.mjs is the explicit page/asset publication allowlist. sync-docs parses
Markdown links/references/images, rewrites included docs to site routes, copies
only allowed public assets and links historical/source-only files to GitHub.
It generates ignored src/content/docs and public/shared; never edit those.
Frontmatter points Edit this page to the canonical source. Site navigation is
explicit in astro.config.mjs. Add a route and CI path filter together.

GitHub Markdown stays usable without Astro frontmatter or duplicate copies.
Historical reports and detailed Firefox research remain in the repository. No recursive docs/state copying occurs. Generated keys/profiles/logs are
excluded. The homepage is a tailored Astro page; operator instructions are shared.

The dev server watches shared docs/assets and rebuilds generated content on edits.
All generated output, search caches, dependencies and browser artifacts are ignored.

## Deployment

See [the maintainer guide](../docs/website-deployment.md) for exact settings.
Deployment defaults disabled. Direct Upload in GitHub Actions is the sole
automatic mechanism, main-only and serialized, without PR previews. The separate
deployment job consumes checked dist/, compares Git ancestry against retained
successful production history, skips superseded sources and verifies full commit
identity. Reruns, unknown history and rollback are documented in the guide.
Production is deployed at https://qwacamole.com/ with main-only automatic
uploads enabled. The maintainer guide records setup, source verification and
rollback. Website publication does not publish the alpha's modules or packages.
