import type { Handle } from '@sveltejs/kit';

// Header halaman dinamis disetel di hook ini. Header worker statis wajib
// disetel terpisah di vercel.json karena aset tidak melewati hook SvelteKit.
import { dev } from '$app/environment';
import { env } from '$env/dynamic/public';

let connectSrc = "connect-src 'self' https://api.algohub.web.id https://*.supabase.co wss://*.supabase.co https://cdn.jsdelivr.net https://esm.sh";
// Di dev, izinkan backend lokal (localhost:8080) supaya CORS/CSP tidak menghalangi.
if (dev) {
	connectSrc = "connect-src 'self' http://localhost:8080 http://localhost:8081 http://127.0.0.1:8080 http://127.0.0.1:8081 https://api.algohub.web.id https://*.supabase.co wss://*.supabase.co https://cdn.jsdelivr.net https://esm.sh";
}

// Izinkan origin API yang dikonfigurasi, termasuk URL Vercel sebelum DNS pindah.
if (env.PUBLIC_API_BASE_URL) {
	const apiURL = new URL(env.PUBLIC_API_BASE_URL);
	if (apiURL.protocol === 'https:' || (dev && apiURL.protocol === 'http:')) {
		connectSrc += ` ${apiURL.origin}`;
	}
}

const SECURITY_HEADERS: Record<string, string> = {
	'X-Frame-Options': 'DENY',
	'X-Content-Type-Options': 'nosniff',
	'Referrer-Policy': 'strict-origin-when-cross-origin',
	'Strict-Transport-Security': 'max-age=63072000; includeSubDomains; preload',
	'Permissions-Policy': 'camera=(self), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()',
	// CSP ini harus mengizinkan runtime nyata aplikasi:
	// - cdn.jsdelivr.net : pyodide + wasi (pyodide.worker.js, clang-worker)
	// - esm.sh           : import browser_wasi_shim di static/c-worker.js
	// - *.supabase.co    : storage (gambar soal, PDF) + API
	'Content-Security-Policy': [
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval' https://cdn.jsdelivr.net https://esm.sh blob:",
		"worker-src 'self' blob:",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net",
		"font-src 'self' https://fonts.gstatic.com https://cdn.jsdelivr.net data:",
		"img-src 'self' data: blob: https://*.supabase.co https://ui-avatars.com",
		"media-src 'self' blob:",
		connectSrc,
		"frame-src 'self' https://*.supabase.co https://docs.google.com https://drive.google.com https://maps.google.com https://www.google.com",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"object-src 'none'"
	].join('; ')
};

// Aset WASM/worker statis gak pernah berubah — cache lama biar tiap buka
// halaman gak unduh ulang clang (31MB) + lld (19MB) + sysroot (9MB).
const CACHEABLE_STATIC = /^\/(clang|lld|sysroot\.tar|memfs|shared\.js|clang-worker\.js|c-worker\.js|pyodide\.worker\.js|playground-worker\.js)(\?|$)/;

export const handle: Handle = async ({ event, resolve }) => {
	const response = await resolve(event);

	response.headers.set('X-Hooks-Test', 'hit');  // DEBUG
	if (CACHEABLE_STATIC.test(event.url.pathname)) {
		response.headers.set('Cache-Control', 'public, max-age=31536000, immutable');
	}

	for (const [key, value] of Object.entries(SECURITY_HEADERS)) {
		response.headers.set(key, value);
	}

	// Halaman yang memakai CodeEditor C (clang-wasm + SAB interaktif) harus
	// cross-origin isolated supaya SharedArrayBuffer hidup: /playground,
	// /playground-c, /praktikum (ruang ujian), /info/praktikum.
	// COEP credentialless dipilih (bukan require-corp) supaya resource
	// cross-origin (gambar Supabase, CDN) tetap dimuat tanpa CORP.
	const SAB_PAGES = ['/playground', '/playground-c', '/praktikum', '/info/praktikum'];
	if (SAB_PAGES.some((p) => event.url.pathname.startsWith(p))) {
		response.headers.set('Cross-Origin-Opener-Policy', 'same-origin');
		response.headers.set('Cross-Origin-Embedder-Policy', 'credentialless');
	}
	return response;
};