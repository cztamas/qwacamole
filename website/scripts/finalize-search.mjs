import * as pagefind from 'pagefind';
import { mkdir, rm, writeFile, readFile, stat } from 'node:fs/promises';
import path from 'node:path';

// Keep Starlight's built-in Pagefind UI/index format, but complete every asset
// write in Node before shutdown. The native writer emitted zero-byte files on
// macOS even after writeFiles() resolved and the integration closed its service.
function checked(response) {
  if (response.errors?.length) throw new Error(response.errors.join('\n'));
  return response;
}
let files;
try {
  const { index } = checked(await pagefind.createIndex());
  checked(await index.addDirectory({ path: 'dist' }));
  files = checked(await index.getFiles()).files;
  await index.deleteIndex();
} finally {
  await pagefind.close();
}
if (!files?.length) throw new Error('Empty static search index');
const output = path.resolve('dist/pagefind');
await rm(output, { recursive: true, force: true });
for (const file of files) {
  const target = path.resolve(output, file.path);
  if (!target.startsWith(output + path.sep) || !file.content.length)
    throw new Error('Invalid/empty static search asset: ' + file.path);
  await mkdir(path.dirname(target), { recursive: true });
  await writeFile(target, file.content);
  if ((await stat(target)).size !== file.content.length) throw new Error('Incomplete search asset: ' + file.path);
}
const entry = JSON.parse(await readFile(path.join(output, 'pagefind-entry.json'), 'utf8'));
if (!(entry.languages?.en?.page_count > 0)) throw new Error('Missing English search pages');
console.log('PASS static search: ' + entry.languages.en.page_count + ' indexed pages; ' + files.length + ' complete assets');
