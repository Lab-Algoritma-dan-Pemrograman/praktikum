import { sveltekit } from '@sveltejs/kit/vite';
import adapter from '@sveltejs/adapter-vercel';
import { defineConfig } from 'vite';
const testEnvDir =
  process.env.ALGOHUB_ENV === 'local' &&
  process.env.ALGOHUB_TEST_MODE === 'true'
    ? process.env.ALGOHUB_TEST_ENV_DIR
    : undefined;
export default defineConfig({
  envDir: testEnvDir,
  envPrefix: ['VITE_', 'PUBLIC_'],
  plugins: [sveltekit({ adapter: adapter(), env: { dir: testEnvDir ?? '.' } })]
});
