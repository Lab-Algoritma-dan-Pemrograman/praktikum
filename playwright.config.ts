import { defineConfig, devices } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import {
  apiOrigin,
  hosts,
  scrubbedEnvironment,
  systemEnvironment
} from './tests/frontend/local';

const root = fileURLToPath(new URL('.', import.meta.url));
const launcher = fileURLToPath(
  new URL('./tests/frontend/serve.mjs', import.meta.url)
);

export default defineConfig({
  testDir: './tests/frontend/e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [['list'], ['html', { open: 'never' }]],
  outputDir: './tests/frontend/results',
  use: {
    ...devices['Desktop Chrome'],
    baseURL: hosts.portal,
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    trace: 'off',
    video: 'off',
    screenshot: 'off',
    launchOptions: {
      env: systemEnvironment(),
      args: [
        '--host-resolver-rules=MAP *.localhost 127.0.0.1',
        '--no-proxy-server'
      ]
    }
  },
  webServer: [
    {
      command: `node "${launcher}" api`,
      cwd: root,
      env: scrubbedEnvironment(),
      url: `${apiOrigin}/healthz`,
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: 'ignore',
      stderr: 'pipe'
    },
    ...Object.entries(hosts).map(([name, origin]) => ({
      command: `node "${launcher}" ${name}`,
      cwd: root,
      env: scrubbedEnvironment(),
      url: `http://127.0.0.1:${new URL(origin).port}/@vite/client`,
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: 'ignore' as const,
      stderr: 'pipe' as const
    }))
  ]
});
