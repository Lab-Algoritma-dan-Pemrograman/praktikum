import type { ServerLoad } from '@sveltejs/kit';
import { loadAppIdentity } from '@algohub/auth-client/server';

export const load: ServerLoad = async (event) => {
  const { principal, applications } = await loadAppIdentity(event);
  return { applications, signedIn: principal !== null };
};
