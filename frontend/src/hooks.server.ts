import type { Handle } from '@sveltejs/kit';

// MEDIUM-05: header keamanan untuk algohub.web.id (praktikum frontend).
// CATATAN: adapter-vercel menulis .vercel/output/config.json TANPA key
// "headers", jadi `headers` di vercel.json TIDAK berpengaruh di sini —
// satu-satunya jalur yang benar adalah hooks.server.ts.
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
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com data:",
		"img-src 'self' data: blob: https://*.supabase.co https://ui-avatars.com",
		"media-src 'self' blob:",
		"connect-src 'self' https://api.algohub.web.id https://*.supabase.co wss://*.supabase.co https://cdn.jsdelivr.net https://esm.sh",
		"frame-src 'self' https://*.supabase.co https://docs.google.com https://drive.google.com https://maps.google.com https://www.google.com",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"object-src 'none'"
	].join('; ')
};

export const handle: Handle = async ({ event, resolve }) => {
	const response = await resolve(event);

	for (const [key, value] of Object.entries(SECURITY_HEADERS)) {
		response.headers.set(key, value);
	}

	// Hanya /playground yang di-isolasi (COOP/COEP) agar SharedArrayBuffer aktif
	// untuk runner Python interaktif. Halaman lain TIDAK ikut, supaya resource
	// cross-origin (gambar Supabase, dll) tetap dimuat normal.
	if (event.url.pathname.startsWith('/playground')) {
		response.headers.set('Cross-Origin-Opener-Policy', 'same-origin');
		response.headers.set('Cross-Origin-Embedder-Policy', 'credentialless');
	}
	return response;
};