import type { ServerLoad } from '@sveltejs/kit';
import { identityData } from '@algohub/auth-client/server';
import type { UserProfile } from '@algohub/auth-client';

export const load: ServerLoad = async (event) => {
  await event.parent();
  return {
    overview: await identityData<{
      user: UserProfile;
      domains: Record<string, boolean>;
    }>(event, '/v1/admin/overview')
  };
};
