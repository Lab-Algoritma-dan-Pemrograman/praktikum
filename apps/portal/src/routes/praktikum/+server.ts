import { redirect, type RequestHandler } from '@sveltejs/kit';

export const GET: RequestHandler = ({ url }) => {
  const target = new URL('https://praktikum.algohub.web.id/');
  target.search = url.search;
  throw redirect(307, target.toString(), { external: true });
};
