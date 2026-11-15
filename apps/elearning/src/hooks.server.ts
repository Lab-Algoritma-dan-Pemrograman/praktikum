import type { RequestEvent } from '@sveltejs/kit';

type Handle = (input: {
  event: RequestEvent;
  resolve: (event: RequestEvent) => Response | Promise<Response>;
}) => Response | Promise<Response>;

// Halaman playground interaktif butuh cross-origin isolation supaya
// SharedArrayBuffer hidup (input() blocking di worker). COEP credentialless
// dipilih (bukan require-corp) supaya runtime Pyodide dari CDN publik
// (jsdelivr mengirim Cross-Origin-Resource-Policy: cross-origin) tetap dimuat.
const SAB_PAGES = ['/playground'];

export const handle: Handle = async ({ event, resolve }) => {
  const response = await resolve(event);
  if (SAB_PAGES.some((path) => event.url.pathname.startsWith(path))) {
    response.headers.set('Cross-Origin-Opener-Policy', 'same-origin');
    response.headers.set('Cross-Origin-Embedder-Policy', 'credentialless');
  }
  return response;
};
