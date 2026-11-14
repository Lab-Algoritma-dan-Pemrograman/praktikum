import type { RequestHandler } from '@sveltejs/kit';
import { proxyIdentity } from '@algohub/auth-client/server';

export const GET: RequestHandler = (event) => proxyIdentity(event, 'elearning');
export const POST = GET;
export const PATCH = GET;
export const PUT = GET;
export const DELETE = GET;
