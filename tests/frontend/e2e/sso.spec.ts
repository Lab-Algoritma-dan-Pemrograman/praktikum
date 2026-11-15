import {
  test,
  expect,
  hosts,
  login,
  api,
  noBrowserCredentials,
  pauseCallback,
  restrictNetwork
} from './helpers';

test('SSO isolates host cookies across all six apps and logout revokes the login family', async ({
  page,
  context
}) => {
  await login(page, 'portal');
  expect(
    (await context.cookies(hosts.admin)).filter(
      (cookie) => cookie.name === 'algohub_session'
    )
  ).toHaveLength(0);
  expect((await page.goto(hosts.admin))?.status()).toBe(401);
  for (const app of ['admin', 'siakad', 'elearning', 'praktikum'] as const) {
    await login(page, app);
    expect((await api(page, '/v1/auth/me')).body.data.user.email).toBe(
      'koordinator@example.test'
    );
    await noBrowserCredentials(page);
  }
  await page.goto(hosts.auth);
  expect((await api(page, '/v1/auth/me')).status).toBe(200);
  const sessions = (await context.cookies()).filter(
    (cookie) => cookie.name === 'algohub_session'
  );
  expect(sessions).toHaveLength(6);
  expect(new Set(sessions.map((cookie) => cookie.value)).size).toBe(6);
  for (const cookie of sessions) {
    expect(cookie.domain.startsWith('.')).toBe(false);
    expect(cookie.httpOnly).toBe(true);
    expect(cookie.sameSite).toBe('Lax');
    expect(cookie.path).toBe('/');
    expect(cookie.secure).toBe(false);
  }
  await page.goto(`${hosts.auth}/logout`);
  expect((await api(page, '/v1/auth/me')).status).toBe(200);
  await page.goto(hosts.siakad);
  await page.getByRole('button', { name: 'Keluar', exact: true }).click();
  await expect(page).toHaveURL(`${hosts.auth}/logout`);
  for (const origin of Object.values(hosts)) {
    await page.goto(origin);
    expect((await api(page, '/v1/auth/me')).status).toBe(401);
  }
});

test('wrong password can be retried without losing the app-bound return target', async ({
  page
}) => {
  await page.goto(`${hosts.admin}/login?return_to=/profil`);
  await page
    .getByLabel('Email atau NIM', { exact: true })
    .fill('koordinator@example.test');
  await page.getByLabel('Password', { exact: true }).fill('wrong-password');
  await page.getByRole('button', { name: 'Masuk', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText(
    'Email atau password tidak sesuai'
  );
  await page
    .getByLabel('Email atau NIM', { exact: true })
    .fill('koordinator@example.test');
  await page.getByLabel('Password', { exact: true }).fill('fixture-password');
  await page.getByRole('button', { name: 'Masuk', exact: true }).click();
  await expect(page).toHaveURL(`${hosts.admin}/profil`);
  await expect(
    page.getByRole('heading', { name: 'Profil dan sesi' })
  ).toBeVisible();
});

test('separate users cannot share identity or logout each other', async ({
  browser
}) => {
  const first = await browser.newContext();
  const second = await browser.newContext();
  await restrictNetwork(first);
  await restrictNetwork(second);
  const coordinator = await first.newPage();
  const student = await second.newPage();
  try {
    await login(coordinator, 'admin');
    await login(student, 'portal', 'mahasiswa@example.test');
    expect((await api(student, '/v1/auth/me')).body.data.user.email).toBe(
      'mahasiswa@example.test'
    );
    expect((await api(coordinator, '/v1/auth/me')).body.data.user.email).toBe(
      'koordinator@example.test'
    );
    expect((await api(coordinator, '/v1/auth/logout', 'POST')).status).toBe(
      200
    );
    expect((await api(coordinator, '/v1/auth/me')).status).toBe(401);
    expect((await api(student, '/v1/auth/me')).status).toBe(200);
  } finally {
    await first.close();
    await second.close();
  }
});

test('callbacks reject absent state, illegal returns, invalid code and replay', async ({
  page,
  context
}) => {
  for (const app of Object.keys(hosts) as Array<keyof typeof hosts>) {
    const response = await page.goto(
      `${hosts[app]}/callback?code=invalid&return_to=${encodeURIComponent(`${hosts[app]}/`)}`
    );
    expect(response?.status()).toBe(400);
    const invalid = await page.goto(
      `${hosts[app]}/api/auth/start?return_to=${encodeURIComponent('//evil.test/')}`
    );
    expect(invalid?.status()).toBe(422);
  }
  await login(page, 'auth');
  let callback = await pauseCallback(page);
  const swapped = new URL(callback);
  swapped.searchParams.set('return_to', `${hosts.admin}/pengaturan`);
  expect((await page.goto(swapped.toString()))?.status()).toBe(400);
  expect(
    (await context.cookies(hosts.admin)).some(
      (cookie) => cookie.name === 'algohub_session'
    )
  ).toBe(false);
  callback = await pauseCallback(page);
  const invalidCode = new URL(callback);
  invalidCode.searchParams.set('code', 'invalid_one_use_code');
  expect((await page.goto(invalidCode.toString()))?.status()).toBe(401);
  callback = await pauseCallback(page);
  expect((await page.goto(callback))?.status()).toBe(200);
  await expect(page).toHaveURL(`${hosts.admin}/`);
  expect((await page.goto(callback))?.status()).toBe(400);
  expect((await api(page, '/v1/auth/me')).status).toBe(200);
});

test('SSO codes require the original host and PKCE verifier; pending login expires', async ({
  page,
  context
}) => {
  await login(page, 'auth');
  let callback = await pauseCallback(page);
  const wrongHost = new URL(callback);
  wrongHost.host = new URL(hosts.siakad).host;
  expect((await page.goto(wrongHost.toString()))?.status()).toBe(400);
  const cookie = (await context.cookies(`${hosts.admin}/callback`)).find(
    (cookie) => cookie.name === 'algohub_sso_pending'
  )!;
  const pending = JSON.parse(Buffer.from(cookie.value, 'base64url').toString());
  await context.addCookies([
    {
      ...cookie,
      value: Buffer.from(
        JSON.stringify({ ...pending, verifier: 'z'.repeat(43) })
      ).toString('base64url')
    }
  ]);
  expect((await page.goto(callback))?.status()).toBe(401);
  callback = await pauseCallback(page);
  const expiring = (await context.cookies(`${hosts.admin}/callback`)).find(
    (cookie) => cookie.name === 'algohub_sso_pending'
  )!;
  const expired = {
    ...JSON.parse(Buffer.from(expiring.value, 'base64url').toString()),
    expiresAt: Date.now() - 1
  };
  await context.addCookies([
    {
      ...expiring,
      value: Buffer.from(JSON.stringify(expired)).toString('base64url')
    }
  ]);
  expect((await page.goto(callback))?.status()).toBe(400);
  expect((await api(page, '/v1/auth/me')).status).toBe(401);
});

test('login CSRF, cookie mutations without CSRF, and unsupported proxy paths are rejected', async ({
  page
}) => {
  await page.goto(hosts.auth);
  const loginStatus = await page.evaluate(
    async () =>
      (
        await fetch('/login?/password', {
          method: 'POST',
          headers: { 'content-type': 'application/x-www-form-urlencoded' },
          body: 'email=koordinator%40example.test&password=fixture-password'
        })
      ).status
  );
  expect(loginStatus).toBe(400);
  await login(page, 'admin');
  expect(
    (
      await api(
        page,
        '/v1/me',
        'PATCH',
        { name: 'CSRF must not update' },
        false
      )
    ).status
  ).toBe(403);
  const badCSRF = await page.evaluate(
    async () =>
      (
        await fetch('/api/v1/auth/logout', {
          method: 'POST',
          headers: {
            'content-type': 'application/json',
            'x-csrf-token': 'incorrect'
          }
        })
      ).status
  );
  expect(badCSRF).toBe(403);
  expect((await api(page, '/v1/auth/me')).status).toBe(200);
  expect((await api(page, '/v1/auth/logout')).status).toBe(405);
  expect(
    (
      await api(page, '/v1/auth/password', 'POST', {
        email: 'koordinator@example.test',
        password: 'fixture-password'
      })
    ).status
  ).toBe(404);
  expect((await api(page, '/v1/healthz')).status).toBe(404);
  const oversized = await api(page, '/v1/me', 'PATCH', {
    name: 'a'.repeat(66_000)
  });
  expect(oversized.status).toBe(413);
});
