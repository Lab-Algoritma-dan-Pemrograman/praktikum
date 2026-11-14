import type { ServerLoad } from '@sveltejs/kit';
import { loadAppIdentity } from '@algohub/auth-client/server';
export const load: ServerLoad = loadAppIdentity;
