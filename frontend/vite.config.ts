import { sveltekit } from '@sveltejs/kit/vite';
import { svelteTesting } from '@testing-library/svelte/vite';
import { defineConfig } from 'vitest/config';

// Worker wasm TIDAK boleh di-serve sebagai 304 (revalidasi cache tanpa header
// COEP → browser blokir: ERR_BLOCKED_BY_RESPONSE). Paksa selalu 200 dengan
// no-cache untuk file worker — lihat juga hooks.server.ts (CACHEABLE_STATIC).
function forceWorker200() {
	return {
		name: 'force-worker-200',
		configureServer(server: any) {
			server.middlewares.use((req: any, res: any, next: any) => {
				if (
					req.url &&
					(req.url.includes('clang-worker.js') ||
						req.url.includes('pyodide.worker.js') ||
						req.url.includes('shared.js') ||
						req.url.includes('c-worker.js'))
				) {
					res.setHeader('Cache-Control', 'no-store, no-cache, must-revalidate');
					res.setHeader('Pragma', 'no-cache');
					res.setHeader('Expires', '0');
					// Worker juga wajib punya COOP/COEP (SAB aktif di dalam worker).
					res.setHeader('Cross-Origin-Opener-Policy', 'same-origin');
					res.setHeader('Cross-Origin-Embedder-Policy', 'credentialless');
					// Hapus conditional headers supaya Vite tidak membalas 304.
					delete req.headers['if-none-match'];
					delete req.headers['if-modified-since'];
				}
				next();
			});
		},
	};
}

export default defineConfig({
	// forceWorker200() WAJIB sebelum sveltekit(): middleware configureServer
	// dipasang sesuai urutan plugin, dan sveltekit middleware menangani request
	// sebelum plugin berikutnya. Kalau forceWorker200 di belakang, static worker
	// (public/clang-worker.js) tidak pernah lewat middleware ini.
	plugins: [forceWorker200(), sveltekit(), svelteTesting()],
	server: {
		port: 5173,
		// COOP/COEP untuk mengaktifkan SharedArrayBuffer (blocking stdin C).
		// credentialless (bukan require-corp) agar resource cross-origin
		// (gambar Supabase, CDN) tetap dimuat tanpa CORP — sejalan dengan
		// hooks.server.ts di production.
		headers: {
			'Cross-Origin-Opener-Policy': 'same-origin',
			'Cross-Origin-Embedder-Policy': 'credentialless'
		}
	},
	test: {
		include: ['src/**/*.{test,spec}.{js,ts}'],
		environment: 'jsdom',
		globals: true,
		css: false,
		setupFiles: ['./vitest-setup.ts'],
		server: {
			deps: {
				inline: [/katex/]
			}
		}
	}
});
