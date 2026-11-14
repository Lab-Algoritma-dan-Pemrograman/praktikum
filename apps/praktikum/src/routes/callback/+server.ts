import type { RequestHandler } from '@sveltejs/kit';
import { completeAppSSO } from '@algohub/auth-client/server';

export const GET: RequestHandler = completeAppSSO;
