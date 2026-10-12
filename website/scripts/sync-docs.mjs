import { readFile, writeFile, mkdir, rm, copyFile, stat } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { unified } from 'unified';
import remarkParse from 'remark-parse';
import remarkStringify from 'remark-stringify';
import { visit } from 'unist-util-visit';
import { documents, assets } from '../docs-map.mjs';

export const website = fileURLToPath(new URL('../', import.meta.url));
export const root = path.resolve(website, '..');
export function resolveLink(value, source) {
  if (/^(?:[a-z][a-z\d+.-]*:|\/|#)/i.test(value)) return value;
  const split = value.search(/[?#]/);
  const local = split === -1 ? value : value.slice(0, split);
  const suffix = split === -1 ? '' : value.slice(split);
  const resolved = path.posix.normalize(path.posix.join(path.posix.dirname(source), decodeURIComponent(local)));
  if (resolved.startsWith('../')) throw new Error('Link escapes repository: ' + value);
  if (documents[resolved]) return '/' + documents[resolved] + '/' + suffix;
  if (assets.includes(resolved)) return '/shared/' + resolved + suffix;
  // Historical evidence/source remain GitHub resources, never silently copied.
  return 'https://github.com/cztamas/qwacamole/blob/main/' + resolved + suffix;
}
export async function syncDocs() {
  const content = path.join(website, 'src/content/docs');
  const shared = path.join(website, 'public/shared');
  await rm(content, { recursive: true, force: true });
  await rm(shared, { recursive: true, force: true });
  const parser = unified().use(remarkParse).use(remarkStringify, { fences: true, bullet: '-' });
  for (const [source, route] of Object.entries(documents)) {
    const tree = parser.parse(await readFile(path.join(root, source), 'utf8'));
    const first = tree.children[0];
    if (first.type !== 'heading' || first.depth !== 1) throw new Error('Expected leading H1: ' + source);
    const title = first.children.map(n => n.value || '').join('');
    tree.children.shift();
    const localLinks = [];
    visit(tree, node => {
      if (['link', 'image', 'definition'].includes(node.type)) {
        if (!/^(?:[a-z][a-z\d+.-]*:|\/|#)/i.test(node.url)) {
          const file = node.url.split(/[?#]/)[0];
          localLinks.push(path.resolve(root, path.dirname(source), decodeURIComponent(file)));
        }
        node.url = resolveLink(node.url, source);
      }
    });
    for (const file of localLinks) await stat(file);
    const frontmatter = '---\ntitle: ' + JSON.stringify(title) +
      '\neditUrl: ' + JSON.stringify('https://github.com/cztamas/qwacamole/edit/main/' + source) + '\n---\n\n';
    const output = path.join(content, route + '.md');
    await mkdir(path.dirname(output), { recursive: true });
    await writeFile(output, frontmatter + parser.stringify(tree));
  }
  for (const source of assets) {
    const output = path.join(shared, source);
    await mkdir(path.dirname(output), { recursive: true });
    await copyFile(path.join(root, source), output);
  }
  console.log('Shared documentation: ' + Object.keys(documents).length + ' pages, ' + assets.length + ' public assets');
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  // Astro caches rendered Markdown by content; start CLI builds/checks fresh
  // so renderer/config changes also reach unchanged shared documents.
  await rm(path.join(website, '.astro'), { recursive: true, force: true });
  await syncDocs();
}
