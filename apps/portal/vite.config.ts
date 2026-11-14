import { createReadStream } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { sveltekit } from '@sveltejs/kit/vite';
import adapter from '@sveltejs/adapter-vercel';
import { defineConfig } from 'vite';

const testEnvDir =
  process.env.ALGOHUB_ENV === 'local' &&
  process.env.ALGOHUB_TEST_MODE === 'true'
    ? process.env.ALGOHUB_TEST_ENV_DIR
    : undefined;
const staticRoot = fileURLToPath(new URL('./static', import.meta.url));
const wasmAssets = new Map([
  ['/runtimes/v1/clang/clang', `${staticRoot}/runtimes/v1/clang/clang`],
  ['/runtimes/v1/clang/lld', `${staticRoot}/runtimes/v1/clang/lld`],
  ['/runtimes/v1/clang/memfs', `${staticRoot}/runtimes/v1/clang/memfs`]
]);

export default defineConfig({
  envDir: testEnvDir,
  envPrefix: ['VITE_', 'PUBLIC_'],
  plugins: [
    {
      name: 'public-browser-runtime',
      configureServer(server) {
        server.middlewares.use((request, response, next) => {
          const pathname = (request.url ?? '').split('?')[0];
          if (pathname.startsWith('/runtimes/v1/')) {
            response.setHeader('Access-Control-Allow-Origin', '*');
            response.setHeader('Cross-Origin-Resource-Policy', 'cross-origin');
            response.setHeader(
              'Cache-Control',
              'public, max-age=31536000, immutable'
            );
          }
          if (
            pathname.startsWith('/runtimes/v2/') ||
            pathname.startsWith('/runtimes/v3/')
          ) {
            response.setHeader(
              'Cache-Control',
              'public, max-age=31536000, immutable'
            );
            response.setHeader('Cross-Origin-Opener-Policy', 'same-origin');
            response.setHeader('Cross-Origin-Embedder-Policy', 'require-corp');
            response.setHeader('Cross-Origin-Resource-Policy', 'cross-origin');
          }
          const wasm = wasmAssets.get(pathname);
          if (wasm) {
            response.setHeader('Content-Type', 'application/wasm');
            response.setHeader('X-Content-Type-Options', 'nosniff');
            const stream = createReadStream(wasm);
            stream.on('error', () => {
              response.statusCode = 404;
              response.end();
            });
            stream.pipe(response);
            return;
          }
          next();
        });
      }
    },
    sveltekit({ adapter: adapter(), env: { dir: testEnvDir ?? '.' } })
  ]
});
