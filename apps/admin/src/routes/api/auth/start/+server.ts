import type { RequestHandler } from '@sveltejs/kit';
import { startAppSSO } from '@algohub/auth-client/server';

export const GET: RequestHandler = startAppSSO;
