import type { ServerLoad } from '@sveltejs/kit';
import { identityData } from '@algohub/auth-client/server';
import type { PermissionDefinition, UserProfile } from '@algohub/auth-client';

export const load: ServerLoad = async (event) => {
  await event.parent();
  const [users, catalog] = await Promise.all([
    identityData<UserProfile[]>(event, '/v1/admin/users?limit=25&offset=0'),
    identityData<PermissionDefinition[]>(event, '/v1/admin/permission-catalog')
  ]);
  return { users, catalog };
};
