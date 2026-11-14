import type { ServerLoad } from '@sveltejs/kit';
import { identityEnvelope } from '@algohub/auth-client/server';
import type { AuditEvent } from '@algohub/auth-client';

export const load: ServerLoad = async (event) => {
  await event.parent();
  const result = await identityEnvelope<AuditEvent[]>(
    event,
    '/v1/admin/auditlog?limit=50'
  );
  return {
    audit: { events: result.data, nextBefore: result.meta.next_before ?? 0 }
  };
};
