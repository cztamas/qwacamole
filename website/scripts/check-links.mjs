import { readdir, readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { load } from 'cheerio';
const root = path.resolve('dist');
async function walk(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  return (await Promise.all(entries.map(e => e.isDirectory() ? walk(path.join(dir, e.name)) : [path.join(dir, e.name)]))).flat();
}
const files = await walk(root);
const pages = new Map();
for (const file of files.filter(f => f.endsWith('.html'))) pages.set(file, load(await readFile(file, 'utf8')));
const errors = [];
let references = 0;
for (const [file, $] of pages) {
  const route = '/' + path.relative(root, file).replace(/index\.html$/, '');
  const base = 'https://qwacamole.com' + route;
  for (const node of $('[href], [src], [srcset]').toArray()) {
    const el = $(node);
    const values = [el.attr('href'), el.attr('src'), ...(el.attr('srcset') || '').split(',').map(v => v.trim().split(/\s/)[0])].filter(Boolean);
    for (const value of values) {
      if (/^(?:mailto:|tel:|data:|javascript:)/.test(value)) continue;
      const url = new URL(value, base);
      if (url.origin !== 'https://qwacamole.com') continue;
      references++;
      const target = path.join(root, decodeURIComponent(url.pathname));
      let resolved;
      try { resolved = (await stat(target)).isDirectory() ? path.join(target, 'index.html') : target; await stat(resolved); }
      catch { errors.push(route + ' → missing ' + value); continue; }
      if (url.hash && pages.has(resolved)) {
        const id = decodeURIComponent(url.hash.slice(1));
        if (!pages.get(resolved)('[id]').toArray().some(n => pages.get(resolved)(n).attr('id') === id))
          errors.push(route + ' → missing anchor ' + value);
      }
    }
  }
}
if (files.some(f => /(?:_worker\.js|\/functions\/|\.pem$|\.key$|\.log$)/.test(f))) errors.push('Unexpected executable/private output');
if (errors.length) throw new Error(errors.join('\n'));
console.log('PASS ' + pages.size + ' HTML pages; ' + references + ' internal links, anchors and assets');
