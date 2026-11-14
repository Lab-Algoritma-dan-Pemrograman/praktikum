import type { ServerLoad } from '@sveltejs/kit';
import { identityData } from '@algohub/auth-client/server';
import type { SessionView, UserProfile } from '@algohub/auth-client';

export const load: ServerLoad = async (event) => {
  await event.parent();
  const [profile, sessions] = await Promise.all([
    identityData<UserProfile>(event, '/v1/me'),
    identityData<SessionView[]>(event, '/v1/auth/sessions')
  ]);
  return { profile, sessions };
};
