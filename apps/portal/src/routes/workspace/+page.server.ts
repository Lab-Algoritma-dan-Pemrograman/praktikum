import { redirect } from '@sveltejs/kit';
import type { ServerLoad } from '@sveltejs/kit';
import { loadAppIdentity } from '@algohub/auth-client/server';

export const load: ServerLoad = async (event) => {
  const data = await loadAppIdentity(event);
  if (!data.principal)
    throw redirect(303, '/api/auth/start?return_to=%2Fworkspace');
  return { ...data, principal: data.principal };
};
