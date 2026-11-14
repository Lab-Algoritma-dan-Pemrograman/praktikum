import { redirect, type RequestHandler } from '@sveltejs/kit';

export const GET: RequestHandler = ({ url }) => {
  throw redirect(
    303,
    `/api/auth/start?${new URLSearchParams({ return_to: url.searchParams.get('return_to') ?? '/' })}`
  );
};
