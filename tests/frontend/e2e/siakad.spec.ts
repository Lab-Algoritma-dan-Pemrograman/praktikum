import { randomUUID } from 'node:crypto';
import type { Browser, Page } from '@playwright/test';
import type { UserAccess } from '@algohub/auth-client';
import {
  test,
  expect,
  hosts,
  login,
  api,
  fetchLocalRoute,
  restrictNetwork,
  noBrowserCredentials
} from './helpers';

async function newActor(browser: Browser, email: string, width = 1280) {
  const context = await browser.newContext({
    viewport: { width, height: 900 },
    reducedMotion: 'reduce'
  });
  await restrictNetwork(context);
  const page = await context.newPage();
  const app =
    email === 'mahasiswa@example.test' || email === 'peminjam@example.test'
      ? 'siakad'
      : 'admin';
  await login(page, app, email);
  return { context, page };
}

async function assertFits(page: Page) {
  const dimensions = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth,
    controls: Array.from(
      document.querySelectorAll<HTMLElement>(
        '.siakad button, .siakad textarea, .siakad select, .siakad input:not([type="checkbox"])'
      )
    )
      .filter((element) => element.checkVisibility())
      .map((element) => {
        const { x, width, height } = element.getBoundingClientRect();
        return { x, width, height };
      })
  }));
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport + 1);
  for (const box of dimensions.controls) {
    expect(box.x).toBeGreaterThanOrEqual(-1);
    expect(box.x + box.width).toBeLessThanOrEqual(dimensions.viewport + 1);
    expect(box.height).toBeGreaterThanOrEqual(43);
  }
}

for (const width of [320, 768, 1280]) {
  test(`siakad QR attendance, inventory rental, division workflow and report at ${width}px`, async ({
    page: admin,
    browser
  }) => {
    test.setTimeout(120_000);
    await admin.setViewportSize({ width, height: 900 });
    await admin.emulateMedia({ reducedMotion: 'reduce' });
    await login(admin, 'admin');
    await admin.goto(`${hosts.admin}/siakad`);
    await expect(
      admin.getByRole('heading', { name: 'Konfigurasi SIAKAD & laboratorium' })
    ).toBeVisible();
    await assertFits(admin);

    // Open QR attendance session as admin/sekretaris.
    await admin.getByLabel('Judul pertemuan').fill(`Pertemuan E2E ${width}px`);
    await admin
      .getByRole('button', { name: 'Buka sesi presensi QR', exact: true })
      .click();
    const qrToken = (
      await admin.getByTestId('qr-token-output').textContent()
    )?.trim();
    expect(qrToken).toBeTruthy();
    await admin.getByRole('button', { name: 'Sembunyikan token QR' }).click();

    const student = await newActor(browser, 'mahasiswa@example.test', width);
    const page = student.page;
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    try {
      await expect(
        page.getByRole('heading', { name: 'SIAKAD', exact: true })
      ).toBeVisible();
      await assertFits(page);

      // Student checks in with QR token.
      await page
        .getByRole('link', { name: 'Presensi QR', exact: true })
        .click();
      await page.getByLabel('Token QR presensi').fill(qrToken!);
      await page.getByRole('button', { name: 'Kirim presensi QR' }).click();
      await expect(
        page.getByText('Presensi berhasil dicatat', { exact: false })
      ).toBeVisible();
      await assertFits(page);

      // Admin validates attendance and closes session.
      await admin
        .getByRole('button', { name: 'Perbarui presensi', exact: true })
        .click();
      const pendingAttendance = admin
        .locator('[data-admin-attendance]')
        .filter({ hasText: 'Mahasiswa Fixture' })
        .filter({ hasText: 'Belum divalidasi' });
      await expect(pendingAttendance).toHaveCount(1);
      await pendingAttendance
        .getByRole('button', { name: /Validasi presensi Mahasiswa Fixture/ })
        .click();
      await expect(admin.getByRole('status')).toContainText(
        'berhasil divalidasi'
      );

      await admin.getByRole('status').waitFor({ state: 'visible' });
      await admin
        .getByRole('button', { name: 'Tutup sesi presensi' })
        .first()
        .click();
      await expect(admin.getByRole('status')).toContainText('ditutup');
      await expect(
        admin.getByRole('button', { name: 'Tutup sesi presensi' })
      ).toHaveCount(0);

      // Student requests an equipment rental.
      await page
        .getByRole('link', { name: 'Inventaris & Penyewaan', exact: true })
        .click();
      await page
        .getByLabel('Keperluan peminjaman')
        .fill(`Praktikum E2E ${width}px`);
      await page.getByRole('button', { name: 'Ajukan peminjaman' }).click();
      await expect(
        page.getByText('berhasil diajukan', { exact: false })
      ).toBeVisible();
      await assertFits(page);

      // Admin switches to Inventaris & Penyewaan tab, approves, verifies payment, and returns.
      await admin
        .getByRole('button', { name: 'Inventaris & Penyewaan', exact: true })
        .click();
      await admin
        .getByRole('button', { name: /Setujui sewa Mahasiswa Fixture/ })
        .first()
        .click();
      await expect(admin.getByRole('status')).toContainText('approved');
      await admin
        .getByRole('button', { name: 'Verifikasi pembayaran' })
        .first()
        .click();
      await expect(
        admin.getByText('Pembayaran terverifikasi').first()
      ).toBeVisible();
      await admin
        .getByRole('button', { name: 'Tandai dikembalikan' })
        .first()
        .click();
      await expect(admin.getByRole('status')).toContainText('returned');
      await expect(
        admin.getByRole('button', { name: 'Tandai dikembalikan' })
      ).toHaveCount(0);
      await assertFits(admin);

      // Check Reports & CSV Export tab.
      await admin
        .getByRole('button', { name: 'Laporan & Export', exact: true })
        .click();
      await expect(
        admin.getByRole('region', { name: 'Export CSV laporan SIAKAD' })
      ).toContainText('total_verified_revenue');
      const downloadEvent = admin.waitForEvent('download');
      await admin.getByRole('link', { name: 'Unduh laporan CSV' }).click();
      const download = await downloadEvent;
      expect(download.suggestedFilename()).toBe('laporan-siakad.csv');
      expect(await download.failure()).toBeNull();
      await assertFits(admin);

      // Student receives notifications on overview page.
      await page
        .getByRole('link', { name: 'Kelas & Jadwal', exact: true })
        .click();
      await expect(
        page.getByRole('region', { name: 'Notifikasi SIAKAD' })
      ).toContainText('Status Penyewaan Alat');
      await noBrowserCredentials(page);
      expect(errors).toEqual([]);
    } finally {
      await student.context.close();
    }
  });
}

test('siakad division-specific UI and API boundaries across sekretaris, k3, bendahara and peminjam', async ({
  browser
}) => {
  test.setTimeout(120_000);
  const k3 = await newActor(browser, 'k3@example.test');
  const bendahara = await newActor(browser, 'bendahara@example.test');
  const peminjam = await newActor(browser, 'peminjam@example.test');
  try {
    // Peminjam requests a rental on SIAKAD host.
    await peminjam.page.goto(`${hosts.siakad}/penyewaan`);
    const purpose = `Sewa Peminjam ${randomUUID().slice(0, 6)}`;
    await peminjam.page.getByLabel('Keperluan peminjaman').fill(purpose);
    await peminjam.page
      .getByRole('button', { name: 'Ajukan peminjaman' })
      .click();
    await expect(
      peminjam.page.getByText('berhasil diajukan', { exact: false })
    ).toBeVisible();

    await bendahara.page.goto(`${hosts.admin}/siakad`);
    const requested = bendahara.page
      .locator('[data-admin-rental]')
      .filter({ hasText: purpose });
    await expect(requested).toBeVisible();
    await expect(
      requested.getByRole('button', { name: 'Verifikasi pembayaran' })
    ).toHaveCount(0);
    const rentalId = await requested.getAttribute('data-admin-rental');
    const prematurePayment = await api(
      bendahara.page,
      `/v1/admin/siakad/penyewaan/${rentalId}`,
      'PATCH',
      { version: 1, action: 'verify_payment' },
      true,
      randomUUID()
    );
    expect(prematurePayment.status).toBe(409);
    expect(prematurePayment.body.error.code).toBe('CONFLICT');

    // K3 opens Admin SIAKAD: sees Inventaris & Penyewaan, does not see Jadwal & Presensi or Kelas.
    await k3.page.goto(`${hosts.admin}/siakad`);
    await expect(
      k3.page.getByRole('button', { name: 'Jadwal & Presensi' })
    ).toHaveCount(0);
    await expect(
      k3.page.getByRole('button', { name: 'Kelas', exact: true })
    ).toHaveCount(0);
    await expect(
      k3.page.getByRole('heading', {
        name: 'Kelola inventaris laboratorium (K3 / Labops)'
      })
    ).toBeVisible();
    await k3.page
      .getByRole('button', { name: /Setujui sewa Peminjam Fixture/ })
      .first()
      .click();
    await expect(k3.page.getByRole('status')).toContainText('approved');

    // Bendahara opens Admin SIAKAD: can verify payment, cannot edit inventory items.
    await bendahara.page.goto(`${hosts.admin}/siakad`);
    await expect(
      bendahara.page.getByRole('heading', {
        name: 'Kelola inventaris laboratorium (K3 / Labops)'
      })
    ).toHaveCount(0);
    await bendahara.page
      .getByRole('button', { name: 'Verifikasi pembayaran' })
      .first()
      .click();
    await expect(
      bendahara.page.getByText('Pembayaran terverifikasi').first()
    ).toBeVisible();
  } finally {
    await k3.context.close();
    await bendahara.context.close();
    await peminjam.context.close();
  }
});

test('siakad rental retry after a lost committed response creates only one request', async ({
  browser
}) => {
  const student = await newActor(browser, 'mahasiswa@example.test');
  const purpose = `Retry ${randomUUID()}`;
  const requests: Array<{ body: string; key: string }> = [];
  let loseResponse = true;
  try {
    await student.page.goto(`${hosts.siakad}/penyewaan`);
    await student.page.route('**/api/v1/inventory/rentals', async (route) => {
      if (route.request().method() !== 'POST') return route.continue();
      requests.push({
        body: route.request().postData()!,
        key: route.request().headers()['idempotency-key']
      });
      if (!loseResponse) return route.continue();
      loseResponse = false;
      const response = await fetchLocalRoute(route);
      expect(response.status()).toBe(201);
      await route.abort('failed');
    });
    await student.page.getByLabel('Keperluan peminjaman').fill(purpose);
    await student.page
      .getByRole('button', { name: 'Ajukan peminjaman' })
      .click();
    await expect(student.page.getByRole('alert')).toContainText(
      'Koneksi SIAKAD terputus'
    );
    await student.page
      .getByRole('button', { name: 'Ajukan peminjaman' })
      .click();
    await expect(student.page.getByRole('status')).toContainText(
      'berhasil diajukan'
    );
    expect(requests).toHaveLength(2);
    expect(requests[1]).toEqual(requests[0]);
    const rentals = (await api(student.page, '/v1/inventory/rentals')).body
      .data;
    expect(
      rentals.filter(
        (rental: { purpose: string }) => rental.purpose === purpose
      )
    ).toHaveLength(1);
    await noBrowserCredentials(student.page);
  } finally {
    await student.context.close();
  }
});

test('siakad one-time QR response loss is reconciled without exposing or duplicating tokens', async ({
  page
}) => {
  await login(page, 'admin');
  await page.goto(`${hosts.admin}/siakad`);
  const title = `Lost QR ${randomUUID()}`;
  const requests: Array<{ body: string; key: string }> = [];
  let loseResponse = true;
  await page.route('**/api/v1/admin/siakad/absensi/sesi', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    requests.push({
      body: route.request().postData()!,
      key: route.request().headers()['idempotency-key']
    });
    if (!loseResponse) return route.continue();
    loseResponse = false;
    const response = await fetchLocalRoute(route);
    expect(response.status()).toBe(201);
    await route.abort('failed');
  });
  await page.getByLabel('Judul pertemuan').fill(title);
  await page
    .getByRole('button', { name: 'Buka sesi presensi QR', exact: true })
    .click();
  await expect(page.getByRole('alert')).toContainText(
    'Koneksi SIAKAD terputus'
  );
  await page
    .getByRole('button', { name: 'Buka sesi presensi QR', exact: true })
    .click();
  await expect(page.getByRole('status')).toContainText(
    'token sekali-pakai tidak dapat diambil ulang'
  );
  await expect(page.getByTestId('qr-token-output')).toHaveCount(0);
  expect(requests).toHaveLength(2);
  expect(requests[1]).toEqual(requests[0]);
  const schedules = (await api(page, '/v1/admin/siakad/jadwal')).body.data;
  expect(
    schedules.filter(
      (s: { open_session: { title: string } | null }) =>
        s.open_session?.title === title
    )
  ).toHaveLength(1);
  const session = page
    .locator('[data-admin-schedule]')
    .filter({ hasText: title });
  await session.getByRole('button', { name: 'Tutup sesi presensi' }).click();
  await expect(page.getByRole('status')).toContainText('ditutup');
});

test('siakad scoped admin forms work outside lab and reject invalid roster IDs before sending', async ({
  page,
  browser
}) => {
  test.setTimeout(120_000);
  await login(page, 'admin');
  await page.goto(`${hosts.admin}/siakad`);
  await page.getByRole('button', { name: 'Kelas', exact: true }).click();
  await expect(page.getByLabel('ID mahasiswa kelas')).toHaveValue('');
  await expect(page.getByLabel('ID asisten kelas')).toHaveValue('');
  const institution = `campus-${randomUUID().slice(0, 8)}`;
  const code = `CLASS-${randomUUID().slice(0, 8)}`;
  await page.getByLabel('Institusi kelas (opsional)').fill(institution);
  await page.getByLabel('Kode kelas', { exact: true }).fill(code);
  await page.getByLabel('Kode mata kuliah').fill('IF-UI');
  await page.getByLabel('Nama mata kuliah').fill('Scope UI');
  await page.getByLabel('ID mahasiswa kelas').fill('1, invalid');
  let writes = 0;
  page.on('request', (request) => {
    if (
      new URL(request.url()).pathname === '/api/v1/admin/siakad/kelas' &&
      request.method() === 'POST'
    )
      writes++;
  });
  await page.getByRole('button', { name: 'Buat kelas', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText(
    'ID anggota harus berupa bilangan bulat positif'
  );
  expect(writes).toBe(0);
  await page.getByLabel('ID mahasiswa kelas').fill('1');
  await page.getByRole('button', { name: 'Buat kelas', exact: true }).click();
  await expect(page.locator('.notice[role="status"]')).toContainText(
    'Data kelas berhasil disimpan'
  );
  const classes = (await api(page, '/v1/admin/siakad/kelas')).body.data;
  expect(
    classes.find((cls: { code: string }) => cls.code === code).scope.institution
  ).toBe(institution);

  const users = (await api(page, '/v1/admin/users?limit=100')).body.data;
  const k3User = users.find(
    (user: { email: string }) => user.email === 'k3@example.test'
  );
  const accessPath = `/v1/admin/users/${k3User.id}/access`;
  const original: UserAccess = (await api(page, accessPath)).body.data;
  const update = (access: UserAccess, version: number, campus?: string) => ({
    version,
    assignments: access.assignments
      .filter((a) => a.active)
      .map(({ role, division, scope }) => ({
        role,
        ...(division ? { division } : {}),
        scope: { ...scope, ...(campus ? { institution: campus } : {}) }
      })),
    grants: access.grants.map(({ permission, role, division, scope }) => ({
      permission,
      role,
      ...(division ? { division } : {}),
      scope: { ...scope, ...(campus ? { institution: campus } : {}) }
    }))
  });
  let actor: Awaited<ReturnType<typeof newActor>> | undefined;
  try {
    expect(
      (
        await api(
          page,
          accessPath,
          'PUT',
          update(original, original.version, institution)
        )
      ).status
    ).toBe(200);
    actor = await newActor(browser, 'k3@example.test', 320);
    await actor.page.goto(`${hosts.admin}/siakad`);
    await expect(
      actor.page.getByRole('heading', {
        name: 'Kelola inventaris laboratorium (K3 / Labops)'
      })
    ).toBeVisible();
    await expect(
      actor.page.getByRole('button', { name: 'Buat alat', exact: true })
    ).toBeDisabled();
    await actor.page.getByLabel('Institusi alat (opsional)').fill(institution);
    await actor.page
      .getByLabel('Kode alat', { exact: true })
      .fill(`ITEM-${randomUUID().slice(0, 8)}`);
    await actor.page
      .getByLabel('Nama alat', { exact: true })
      .fill('Scoped inventory');
    await actor.page
      .getByRole('button', { name: 'Buat alat', exact: true })
      .click();
    await expect(actor.page.locator('.notice[role="status"]')).toContainText(
      'Item inventaris berhasil disimpan'
    );
    await actor.page
      .getByLabel('Nama alat', { exact: true })
      .fill('Scoped inventory edited');
    await actor.page
      .getByRole('button', { name: 'Simpan alat', exact: true })
      .click();
    await expect(actor.page.locator('.notice[role="status"]')).toContainText(
      'Item inventaris berhasil disimpan'
    );
    const items = (await api(actor.page, '/v1/inventory/items')).body.data;
    expect(
      items.find(
        (item: { name: string }) => item.name === 'Scoped inventory edited'
      ).scope.institution
    ).toBe(institution);
    await assertFits(actor.page);
    await actor.page
      .getByRole('button', { name: 'Laporan & Export', exact: true })
      .click();
    await expect(actor.page.getByTestId('verified-revenue')).toHaveCount(0);
    await expect(
      actor.page.getByRole('region', { name: 'Export CSV laporan SIAKAD' })
    ).toContainText('total_verified_revenue,0');
    const report = (await api(actor.page, '/v1/admin/siakad/laporan')).body
      .data;
    expect(report.total_verified_revenue).toBe(0);
    await expect(
      actor.page.getByRole('link', { name: 'Unduh laporan CSV' })
    ).toHaveAttribute('download', 'laporan-siakad.csv');
  } finally {
    const current: UserAccess = (await api(page, accessPath)).body.data;
    expect(
      (await api(page, accessPath, 'PUT', update(original, current.version)))
        .status
    ).toBe(200);
    await actor?.context.close();
  }
});
