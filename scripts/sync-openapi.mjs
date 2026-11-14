import { readFile, writeFile } from 'node:fs/promises';
import { parse } from 'yaml';

const source = await readFile('docs/openapi/openapi.yaml', 'utf8');
const document = parse(source, { merge: true });
const json = `${JSON.stringify(document, null, 2)}\n`;
const outputs = [
  ['docs/openapi/openapi.json', json],
  ['backend/internal/platform/openapi/spec.yaml', source],
  ['backend/internal/platform/openapi/spec.json', json]
];
for (const [path, content] of outputs) {
  if (process.argv.includes('--check')) {
    if ((await readFile(path, 'utf8')) !== content)
      throw new Error(`${path} is out of sync`);
  } else {
    await writeFile(path, content);
  }
}
console.log(
  process.argv.includes('--check')
    ? 'OpenAPI bundles are in sync.'
    : 'OpenAPI bundles updated.'
);
