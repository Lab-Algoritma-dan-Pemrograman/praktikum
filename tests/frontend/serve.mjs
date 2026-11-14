import { spawn, execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { fixtureEnvironment, hosts } from './local.ts';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const target = process.argv[2];
if (target !== 'api' && !Object.hasOwn(hosts, target))
  throw new Error('Unknown local test server');
const temporary = mkdtempSync(join(tmpdir(), 'algohub-e2e-'));
const env = { ...fixtureEnvironment(), ALGOHUB_TEST_ENV_DIR: temporary };
let command;
let args;
let cwd;
if (target === 'api') {
  command = join(temporary, process.platform === 'win32' ? 'api.exe' : 'api');
  cwd = join(root, 'backend');
  execFileSync('go', ['build', '-o', command, './cmd/api'], {
    cwd,
    env,
    stdio: 'inherit'
  });
  args = [];
} else {
  command = process.execPath;
  cwd = join(root, 'apps', target);
  args = [
    join(root, 'node_modules/vite/bin/vite.js'),
    '--host',
    '127.0.0.1',
    '--port',
    new URL(hosts[target]).port,
    '--strictPort'
  ];
}
const child = spawn(command, args, { cwd, env, stdio: 'inherit' });
for (const signal of ['SIGINT', 'SIGTERM'])
  process.on(signal, () => child.kill(signal));
child.on('exit', (code) => {
  try {
    rmSync(temporary, { recursive: true, force: true });
  } catch {
    /* Windows may still hold the test executable. */
  }
  process.exit(code ?? 1);
});
child.on('error', () => process.exit(1));
