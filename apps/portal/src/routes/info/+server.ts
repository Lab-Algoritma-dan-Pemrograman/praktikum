import { redirect, type RequestHandler } from '@sveltejs/kit';

export const GET: RequestHandler = () => {
  throw redirect(308, '/');
};
