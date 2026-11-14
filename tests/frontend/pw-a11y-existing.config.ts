import { defineConfig, devices } from '@playwright/test';
import { hosts } from './local';

if (process.env.ALGOHUB_E2E_USE_EXISTING_SERVERS !== 'true')
  throw new Error(
    'Set ALGOHUB_E2E_USE_EXISTING_SERVERS=true only after verifying every configured origin belongs to Algohub.'
  );

// Explicit override only: this config never starts or identifies the target servers.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [['list']],
  outputDir: './results',
  use: {
    ...devices['Desktop Chrome'],
    baseURL: hosts.portal,
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    trace: 'off',
    video: 'off',
    screenshot: 'off',
    launchOptions: {
      args: [
        '--host-resolver-rules=MAP *.localhost 127.0.0.1',
        '--no-proxy-server'
      ]
    }
  }
});
