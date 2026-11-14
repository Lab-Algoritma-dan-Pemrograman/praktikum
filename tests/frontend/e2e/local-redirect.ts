import { request } from 'node:http';
import type { BrowserContext } from '@playwright/test';
import { hosts } from '../local';

export async function localRedirect(
  context: BrowserContext,
  target: string
): Promise<string> {
  const url = new URL(target);
  if (!Object.values(hosts).includes(url.origin as typeof hosts.portal))
    throw new Error('Only local test hosts are allowed');
  const cookies = await context.cookies(target);
  const response = await new Promise<{
    status: number;
    location?: string;
    cookies: string[];
  }>((resolve, reject) => {
    const req = request(
      {
        hostname: '127.0.0.1',
        port: url.port,
        path: `${url.pathname}${url.search}`,
        headers: {
          host: url.host,
          cookie: cookies
            .map((cookie) => `${cookie.name}=${cookie.value}`)
            .join('; ')
        },
        timeout: 10_000
      },
      (res) => {
        res.resume();
        res.on('end', () =>
          resolve({
            status: res.statusCode ?? 0,
            location: res.headers.location,
            cookies: res.headers['set-cookie'] ?? []
          })
        );
      }
    );
    req.on('timeout', () =>
      req.destroy(new Error('Local SSO request timed out'))
    );
    req.on('error', reject);
    req.end();
  });
  if (response.status !== 303 || !response.location)
    throw new Error(`Expected local SSO redirect, received ${response.status}`);
  for (const raw of response.cookies) {
    const [pair, ...parts] = raw.split(';');
    const separator = pair.indexOf('=');
    const name = pair.slice(0, separator);
    if (
      ![
        'algohub_sso_pending',
        'algohub_auth_pending',
        'algohub_session',
        'algohub_csrf'
      ].includes(name)
    )
      continue;
    const attributes = new Map(
      parts.map((part) => {
        const [key, ...value] = part.trim().split('=');
        return [key.toLowerCase(), value.join('=')];
      })
    );
    const path = attributes.get('path') ?? '/';
    if (attributes.get('max-age') === '0') {
      await context.clearCookies({ name, domain: url.hostname, path });
      continue;
    }
    await context.addCookies([
      {
        name,
        value: pair.slice(separator + 1),
        domain: url.hostname,
        path,
        httpOnly: attributes.has('httponly'),
        secure: attributes.has('secure'),
        sameSite: 'Lax',
        ...(attributes.has('max-age')
          ? { expires: Date.now() / 1000 + Number(attributes.get('max-age')) }
          : {})
      }
    ]);
  }
  return response.location;
}
