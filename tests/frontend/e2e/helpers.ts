import {
  test as base,
  expect,
  type BrowserContext,
  type Page,
  type Route
} from '@playwright/test';
import { hosts } from '../local';
import { localRedirect } from './local-redirect';
export { expect, hosts };

export async function fetchLocalRoute(route: Route) {
  const target = new URL(route.request().url());
  if (!Object.values(hosts).includes(target.origin as typeof hosts.portal))
    throw new Error('Only local test hosts are allowed');
  const host = target.host;
  target.hostname = '127.0.0.1';
  try {
    return await route.fetch({
      url: target.href,
      headers: { ...(await route.request().allHeaders()), host },
      maxRedirects: 0,
      timeout: 10000
    });
  } catch {
    throw new Error(
      'Local intercepted request failed; credential headers withheld.'
    );
  }
}

export async function restrictNetwork(context: BrowserContext) {
  await context.route('**/*', (route) =>
    Object.values(hosts).includes(
      new URL(route.request().url()).origin as typeof hosts.portal
    )
      ? route.continue()
      : route.abort()
  );
}

export const test = base.extend({
  context: async ({ context }, use) => {
    await restrictNetwork(context);
    await use(context);
  },
  page: async ({ page }, use) => {
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await use(page);
    expect(errors).toEqual([]);
  }
});

export async function login(
  page: Page,
  app: keyof typeof hosts = 'admin',
  email = 'koordinator@example.test'
) {
  await page.goto(`${hosts[app]}/login`);
  if (
    new URL(page.url()).origin === hosts.auth &&
    new URL(page.url()).pathname === '/login'
  ) {
    await page.getByLabel('Email atau NIM', { exact: true }).fill(email);
    await page.getByLabel('Password', { exact: true }).fill('fixture-password');
    const submission = page.waitForRequest(
      (request) =>
        request.method() === 'POST' &&
        new URL(request.url()).pathname === '/login'
    );
    await page.getByRole('button', { name: 'Masuk', exact: true }).click();
    expect((await (await submission).allHeaders()).origin).toBe(hosts.auth);
  }
  // Login langsung di host auth (tanpa return_to) sengaja berakhir di
  // portal /workspace sejak SSO workspace diperkenalkan.
  const destination =
    app === 'auth'
      ? { origin: hosts.portal, pathname: '/workspace' }
      : { origin: hosts[app], pathname: '/' };
  await expect(page).toHaveURL(
    (url) =>
      url.origin === destination.origin && url.pathname === destination.pathname
  );
}

export async function api(
  page: Page,
  path: string,
  method = 'GET',
  data?: unknown,
  csrf = true,
  key?: string
) {
  return page.evaluate(
    async ({ path, method, data, csrf, key }) => {
      const value =
        document.cookie
          .split(';')
          .map((part) => part.trim())
          .find((part) => part.startsWith('algohub_csrf='))
          ?.slice('algohub_csrf='.length) ?? '';
      const response = await fetch(`/api${path}`, {
        method,
        headers: {
          Accept: 'application/json',
          ...(method !== 'GET'
            ? {
                'Content-Type': 'application/json',
                ...(csrf ? { 'X-CSRF-Token': decodeURIComponent(value) } : {}),
                ...(key ? { 'Idempotency-Key': key } : {})
              }
            : {})
        },
        ...(data !== undefined ? { body: JSON.stringify(data) } : {})
      });
      return { status: response.status, body: await response.json() };
    },
    { path, method, data, csrf, key }
  );
}

export async function noBrowserCredentials(page: Page) {
  const exposed = await page.evaluate(() =>
    [
      location.href,
      document.cookie,
      document.documentElement.innerHTML,
      JSON.stringify({ ...localStorage }),
      JSON.stringify({ ...sessionStorage })
    ].join('\n')
  );
  const cookies = (await page.context().cookies()).filter(
    (cookie) => cookie.name === 'algohub_session'
  );
  for (const cookie of cookies)
    expect(exposed.includes(cookie.value)).toBe(false);
  expect(new URL(page.url()).searchParams.has('access_token')).toBe(false);
  expect(new URL(page.url()).searchParams.has('refresh_token')).toBe(false);
}

export async function pauseCallback(
  page: Page,
  app: keyof typeof hosts = 'admin'
): Promise<string> {
  const loginURL = await localRedirect(
    page.context(),
    `${hosts[app]}/api/auth/start`
  );
  const callback = await localRedirect(page.context(), loginURL);
  expect(new URL(callback).origin).toBe(hosts[app]);
  expect(new URL(callback).pathname).toBe('/callback');
  return callback;
}
