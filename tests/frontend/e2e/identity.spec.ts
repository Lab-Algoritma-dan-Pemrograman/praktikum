import type { UserAccess, SessionView } from '@algohub/auth-client';
import {
  test,
  expect,
  hosts,
  login,
  api,
  noBrowserCredentials,
  restrictNetwork
} from './helpers';

function updateBody(access: UserAccess, version = access.version) {
  return {
    version,
    assignments: access.assignments
      .filter((assignment) => assignment.active)
      .map(({ role, division, scope }) => ({
        role,
        ...(division ? { division } : {}),
        scope
      })),
    grants: access.grants.map(({ permission, role, division, scope }) => ({
      permission,
      role,
      ...(division ? { division } : {}),
      scope
    }))
  };
}
function newest(sessions: SessionView[], host: string) {
  return sessions
    .filter((session) => session.host === host && !session.revoked)
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))[0];
}

test('profile saves live data, retries reads safely, and refresh rotates without extending expiry', async ({
  page,
  context,
  browser
}) => {
  await login(page, 'admin');
  await page.goto(`${hosts.admin}/profil`);
  const original = (await api(page, '/v1/me')).body.data;
  let failOnce = true;
  await page.route('**/api/v1/me', async (route) => {
    if (route.request().method() === 'PATCH' && failOnce) {
      failOnce = false;
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'UNAVAILABLE',
            message: 'Layanan sedang tidak tersedia.'
          },
          meta: {
            request_id: 'fixture-retry',
            operation_id: 'identity.profile.update'
          }
        })
      });
    } else await route.continue();
  });
  try {
    await expect(
      page.getByRole('button', { name: 'Simpan profil' })
    ).toBeEnabled();
    await page.getByLabel('Nama tampilan').fill('Koordinator UI Fixture');
    await page.getByRole('button', { name: 'Simpan profil' }).click();
    await expect(page.getByRole('alert')).toContainText('fixture-retry');
    expect((await api(page, '/v1/me')).body.data.name).toBe(original.name);
    await page.getByRole('button', { name: 'Muat ulang', exact: true }).click();
    await expect(page.getByRole('alert')).toHaveCount(0);
    await expect(page.getByLabel('Nama tampilan')).toHaveValue(original.name);
    await page.getByLabel('Nama tampilan').fill('Koordinator UI Fixture');
    await page.getByRole('button', { name: 'Simpan profil' }).click();
    await expect(page.getByRole('status')).toContainText(
      'Profil berhasil diperbarui'
    );
    await expect(page.locator('.user-name')).toHaveText(
      'Koordinator UI Fixture'
    );
    const beforeCookies = await context.cookies(hosts.admin);
    const before = newest(
      (await api(page, '/v1/auth/sessions')).body.data,
      new URL(hosts.admin).host
    );
    await page.getByRole('button', { name: 'Perbarui token sesi' }).click();
    await expect(page.getByRole('status')).toContainText(
      'tanpa memperpanjang masa berlaku'
    );
    const afterCookies = await context.cookies(hosts.admin);
    for (const name of ['algohub_session', 'algohub_csrf'])
      expect(
        beforeCookies.find((cookie) => cookie.name === name)!.value ===
          afterCookies.find((cookie) => cookie.name === name)!.value
      ).toBe(false);
    const after = newest(
      (await api(page, '/v1/auth/sessions')).body.data,
      new URL(hosts.admin).host
    );
    expect(after.expires_at).toBe(before.expires_at);
    const oldContext = await browser.newContext();
    await restrictNetwork(oldContext);
    try {
      await oldContext.addCookies(beforeCookies);
      const oldPage = await oldContext.newPage();
      expect((await oldPage.goto(hosts.admin))?.status()).toBe(401);
    } finally {
      await oldContext.close();
    }
    await noBrowserCredentials(page);
  } finally {
    await page.unroute('**/api/v1/me');
    await api(page, '/v1/me', 'PATCH', { name: original.name });
  }
});

test('profile revokes another app session and current-session revocation shows expired UX', async ({
  page,
  context
}) => {
  await login(page, 'admin');
  await login(page, 'portal');
  const portalSession = newest(
    (await api(page, '/v1/auth/sessions')).body.data,
    new URL(hosts.portal).host
  );
  await page.goto(`${hosts.admin}/profil`);
  const portalRow = page.locator(`[data-session-id="${portalSession.id}"]`);
  await portalRow.getByRole('button', { name: /^Cabut sesi/ }).click();
  await portalRow.getByRole('button', { name: 'Ya, cabut sesi' }).click();
  await expect(portalRow).toContainText('Sesi telah dicabut');
  const portal = await context.newPage();
  await portal.goto(hosts.portal);
  expect((await api(portal, '/v1/auth/me')).status).toBe(401);
  await portal.close();
  expect((await api(page, '/v1/auth/me')).status).toBe(200);
  const current = newest(
    (await api(page, '/v1/auth/sessions')).body.data,
    new URL(hosts.admin).host
  );
  const currentRow = page.locator(`[data-session-id="${current.id}"]`);
  await currentRow.getByRole('button', { name: /^Cabut sesi/ }).click();
  await currentRow.getByRole('button', { name: 'Ya, cabut sesi' }).click();
  await expect(
    page.getByRole('heading', { name: 'Sesi berakhir atau belum masuk' })
  ).toBeVisible();
  expect((await page.reload())?.status()).toBe(401);
  await expect(page.getByRole('link', { name: 'Masuk kembali' })).toBeVisible();
});

test('coordinator editor rejects self-change and handles optimistic conflict without blind retry', async ({
  page
}) => {
  await login(page, 'admin');
  await page.goto(`${hosts.admin}/pengaturan`);
  const users = (await api(page, '/v1/admin/users?limit=25&offset=0')).body
    .data;
  const me = users.find(
    (user: { email: string }) => user.email === 'koordinator@example.test'
  );
  const target = users.find(
    (user: { email: string }) => user.email === 'pdd@example.test'
  );
  await page
    .getByRole('combobox', { name: 'Pengguna', exact: true })
    .selectOption(String(me.id));
  await page.getByRole('button', { name: 'Muat akses', exact: true }).click();
  await expect(
    page.getByText('Anda tidak dapat mengubah akses sendiri', { exact: false })
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Simpan perubahan akses' })
  ).toBeDisabled();
  await page
    .getByRole('combobox', { name: 'Pengguna', exact: true })
    .selectOption(String(target.id));
  await page.getByRole('button', { name: 'Muat akses', exact: true }).click();
  const original: UserAccess = (
    await api(page, `/v1/admin/users/${target.id}/access`)
  ).body.data;
  try {
    expect(
      (
        await api(
          page,
          `/v1/admin/users/${target.id}/access`,
          'PUT',
          updateBody(original)
        )
      ).status
    ).toBe(200);
    await page.getByRole('button', { name: 'Simpan perubahan akses' }).click();
    await expect(page.getByRole('alert')).toContainText(
      'Data akses telah berubah'
    );
    await expect(
      page.getByRole('button', { name: 'Simpan perubahan akses' })
    ).toBeDisabled();
    await page
      .getByRole('button', { name: 'Muat ulang akses', exact: true })
      .click();
    await expect(
      page.getByRole('button', { name: 'Simpan perubahan akses' })
    ).toBeEnabled();
    await expect(page.getByRole('alert')).toHaveCount(0);
  } finally {
    const current = (await api(page, `/v1/admin/users/${target.id}/access`))
      .body.data;
    expect(
      (
        await api(
          page,
          `/v1/admin/users/${target.id}/access`,
          'PUT',
          updateBody(original, current.version)
        )
      ).status
    ).toBe(200);
  }
});

test('access editor applies role/division/scoped grant, audit paginates empty authorized pages and unassigned UX remains empty', async ({
  page,
  browser
}) => {
  test.setTimeout(120_000);
  await login(page, 'admin');
  await page.goto(`${hosts.admin}/pengaturan`);
  const users = (await api(page, '/v1/admin/users?limit=25&offset=0')).body
    .data;
  const user = users.find(
    (user: { email: string }) => user.email === 'mahasiswa@example.test'
  );
  const original: UserAccess = (
    await api(page, `/v1/admin/users/${user.id}/access`)
  ).body.data;
  const studentContext = await browser.newContext();
  await restrictNetwork(studentContext);
  const student = await studentContext.newPage();
  try {
    await login(student, 'admin', 'mahasiswa@example.test');
    for (let index = 0; index < 55; index++)
      expect(
        (await api(student, '/v1/admin/access?path=/pengaturan')).status
      ).toBe(403);
    await page
      .getByRole('combobox', { name: 'Pengguna', exact: true })
      .selectOption(String(user.id));
    await page.getByRole('button', { name: 'Muat akses', exact: true }).click();
    await page.getByRole('button', { name: 'Tambah penugasan' }).click();
    const assignmentIndex = original.assignments.filter(
      (assignment) => assignment.active
    ).length;
    await page
      .locator(`#assignment-role-${assignmentIndex}`)
      .selectOption('asisten');
    await page
      .locator(`#assignment-division-${assignmentIndex}`)
      .selectOption('sekretaris');
    await page.getByRole('button', { name: 'Tambah grant' }).click();
    const grantIndex = original.grants.length;
    await page
      .locator(`#grant-permission-${grantIndex}`)
      .selectOption('audit.read');
    await page.locator(`#grant-role-${grantIndex}`).selectOption('asisten');
    await page
      .locator(`#grant-division-${grantIndex}`)
      .selectOption('sekretaris');
    await page.locator(`#grant-${grantIndex}-domain`).fill('siakad');
    await page.locator(`#grant-${grantIndex}-class_id`).fill('class-ui');
    await page.getByRole('button', { name: 'Simpan perubahan akses' }).click();
    await expect(page.getByRole('status')).toContainText(
      'Akses berhasil disimpan'
    );
    const updated: UserAccess = (
      await api(page, `/v1/admin/users/${user.id}/access`)
    ).body.data;
    expect(
      updated.assignments.some(
        (assignment) =>
          assignment.role === 'asisten' && assignment.division === 'sekretaris'
      )
    ).toBe(true);
    expect(
      updated.grants.some(
        (grant) =>
          grant.permission === 'audit.read' &&
          grant.scope.class_id === 'class-ui'
      )
    ).toBe(true);
    expect((await student.goto(`${hosts.admin}/auditlog`))?.status()).toBe(200);
    await expect(
      student.getByText('Belum ada event audit dalam scope yang diberikan.')
    ).toBeVisible();
    await expect(
      student.getByRole('button', { name: 'Lebih lama' })
    ).toBeEnabled();
    let failOnce = true;
    await student.route('**/api/v1/admin/auditlog?*', async (route) => {
      if (failOnce) {
        failOnce = false;
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({
            error: { code: 'UNAVAILABLE', message: 'Audit belum tersedia.' },
            meta: { request_id: 'audit-retry', operation_id: 'admin.auditlog' }
          })
        });
      } else await route.continue();
    });
    await student.getByRole('button', { name: 'Lebih lama' }).click();
    await expect(student.getByRole('alert')).toContainText('audit-retry');
    await student
      .getByRole('button', { name: 'Muat ulang', exact: true })
      .click();
    await expect(student.getByRole('alert')).toHaveCount(0);
    await student.getByRole('button', { name: 'Lebih lama' }).click();
    await expect(
      student.getByRole('button', { name: 'Sebelumnya', exact: true })
    ).toBeEnabled();
    await expect(
      student.getByText('Belum ada event audit dalam scope yang diberikan.')
    ).toBeVisible();
    while (await page.getByRole('button', { name: /^Hapus grant / }).count())
      await page
        .getByRole('button', { name: /^Hapus grant / })
        .first()
        .click();
    while (
      await page.getByRole('button', { name: /^Hapus penugasan / }).count()
    )
      await page
        .getByRole('button', { name: /^Hapus penugasan / })
        .first()
        .click();
    await page.getByRole('button', { name: 'Simpan perubahan akses' }).click();
    await expect(page.getByRole('status')).toContainText(
      'Akses berhasil disimpan'
    );
    await login(student, 'portal', 'mahasiswa@example.test');
    // Ringkasan peran pindah dari landing (0ae1db0) ke /workspace (4aa6607).
    await student.goto(`${hosts.portal}/workspace`);
    await expect(
      student.getByText('Belum ada penugasan.', { exact: false })
    ).toBeVisible();
    expect((await student.goto(hosts.admin))?.status()).toBe(403);
  } finally {
    const current = (await api(page, `/v1/admin/users/${user.id}/access`)).body
      .data;
    expect(
      (
        await api(
          page,
          `/v1/admin/users/${user.id}/access`,
          'PUT',
          updateBody(original, current.version)
        )
      ).status
    ).toBe(200);
    await studentContext.close();
  }
});
