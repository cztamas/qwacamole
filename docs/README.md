# Documentation audiences

`guides/` contains canonical manuals for installing, configuring, evaluating and
operating qwacamole. They render on GitHub and the website from the same Markdown.
The real security policy stays at repository-root SECURITY.md for GitHub discovery.

Root-level docs contain engineering architecture, plans, dated reports and
maintainer workflows. In particular `demos.md` retains detailed native acceptance
procedures and `firefox.md` retains browser research. Release preparation,
contribution, website toolchain/CI, Cloudflare and DNS instructions remain
repository material. They are not website documentation routes.

`website/docs-map.mjs` is the full explicit publication boundary: user guides,
SECURITY.md and selected public configuration/licence/image assets. It never
recursively copies docs. Generated keys, profiles, raw logs and dated acceptance
reports are excluded. Website editing/start/preview instructions live in the root
README and website/README; deployment maintenance lives in website-deployment.md.
