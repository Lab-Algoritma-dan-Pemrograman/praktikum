import type { ServerLoad } from '@sveltejs/kit';
import {
  applicationLinks,
  identityConfig,
  identityData,
  privateIdentityPage
} from '@algohub/auth-client/server';
import type { Principal } from '@algohub/auth-client';

export const load: ServerLoad = async (event) => {
  event.depends('identity:access');
  privateIdentityPage(event);
  const config = identityConfig(event.url);
  const principal = await identityData<Principal>(
    event,
    `/v1/admin/access?${new URLSearchParams({ path: event.url.pathname })}`
  );
  return {
    principal,
    applications: applicationLinks(config),
    authOrigin: config.authOrigin
  };
};
