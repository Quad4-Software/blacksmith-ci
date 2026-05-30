import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const distDir = path.resolve('dist');

let entries;
try {
  entries = await fs.readdir(distDir);
} catch (err) {
  if (err.code === 'ENOENT') {
    console.log('[latest] dist/ not found — run astro build first');
    process.exit(0);
  }
  throw err;
}

const versions = entries.filter((name) => /^v\d+-\d+$/.test(name));
if (versions.length === 0) {
  console.log('[latest] no versioned dirs in dist/ — skipping');
  process.exit(0);
}

const sorted = versions.toSorted((a, b) => {
  const [aMaj, aMin] = a.slice(1).split('-').map(Number);
  const [bMaj, bMin] = b.slice(1).split('-').map(Number);
  return bMaj === aMaj ? bMin - aMin : bMaj - aMaj;
});
const latest = sorted[0];

const latestDir = path.join(distDir, latest);
const outDir = path.join(distDir, 'latest');
await fs.rm(outDir, { recursive: true, force: true });

async function walk(dir) {
  const out = [];
  for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...(await walk(full)));
    } else if (entry.isFile() && entry.name.endsWith('.html')) {
      out.push(full);
    }
  }
  return out;
}

const htmlFiles = await walk(latestDir);
let count = 0;
for (const file of htmlFiles) {
  const rel = path.relative(latestDir, file).split(path.sep).join('/');
  const dest = path.join(outDir, ...rel.split('/'));
  await fs.mkdir(path.dirname(dest), { recursive: true });

  let target = `/${latest}/${rel}`;
  if (target.endsWith('/index.html')) {
    target = target.slice(0, -'index.html'.length);
  } else if (target.endsWith('.html')) {
    target = `${target.slice(0, -'.html'.length)}/`;
  }

  const html = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta http-equiv="refresh" content="0; url=${target}" />
    <link rel="canonical" href="${target}" />
    <title>Redirecting…</title>
  </head>
  <body>
    <p>Redirecting to <a href="${target}">${target}</a>…</p>
  </body>
</html>
`;
  await fs.writeFile(dest, html);
  count++;
}
console.log(`[latest] wrote ${count} /latest/* redirect(s) → /${latest}/*`);
