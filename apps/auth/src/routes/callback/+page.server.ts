import type { ServerLoad } from '@sveltejs/kit';
import { completeCentralCallback } from '@algohub/auth-client/server';

export const load: ServerLoad = completeCentralCallback;
