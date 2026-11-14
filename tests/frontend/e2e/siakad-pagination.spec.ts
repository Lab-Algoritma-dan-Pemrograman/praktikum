import type { Page } from '@playwright/test';
import { test, expect, hosts, login, api } from './helpers';

async function mockPages(page: Page, lists: Record<string, unknown[]>) {
  const offsets: Record<string, number[]> = {};
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url());
    const rows = lists[url.pathname];
    if (route.request().method() !== 'GET' || !rows) return route.fallback();
    const offset = Number(url.searchParams.get('offset') ?? 0);
    const limit = Number(url.searchParams.get('limit') ?? 25);
    expect(limit).toBe(25);
    (offsets[url.pathname] ??= []).push(offset);
    await route.fulfill({ json: { data: rows.slice(offset, offset + limit) } });
  });
  return offsets;
}

const numbered = <T extends { id: string }>(row: T, prefix: string) =>
  Array.from({ length: 27 }, (_, i) => ({ ...row, id: `${prefix}-${i}` }));

test('siakad pagination keeps class/item creations and drafts across hydrated admin tabs', async ({
  page
}) => {
  await login(page, 'admin');
  const cls = (await api(page, '/v1/admin/siakad/kelas')).body.data[0];
  const item = (await api(page, '/v1/inventory/items')).body.data[0];
  const offsets = await mockPages(page, {
    '/api/v1/admin/siakad/kelas': numbered(cls, 'class-page'),
    '/api/v1/inventory/items': numbered(item, 'item-page')
  });
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route('**/*', async (route) => {
    if (route.request().resourceType() === 'script') await gate;
    await route.fallback();
  });
  const navigation = page.goto(`${hosts.admin}/siakad`, {
    waitUntil: 'domcontentloaded'
  });
  try {
    await expect(
      page.getByRole('button', { name: 'Kelas', exact: true })
    ).toBeDisabled();
  } finally {
    release();
  }
  await navigation;
  await page.getByRole('button', { name: 'Kelas', exact: true }).click();
  await expect(page.getByLabel('ID mahasiswa kelas')).toHaveValue('');
  await page.getByLabel('Kode kelas', { exact: true }).fill('DRAFT-CLASS');
  await page.getByRole('button', { name: 'Muat kelas lainnya' }).click();
  await expect(
    page.getByLabel('Pilih kelas untuk diedit').locator('option')
  ).toHaveCount(28);
  await expect(page.getByLabel('Kode kelas', { exact: true })).toHaveValue(
    'DRAFT-CLASS'
  );
  expect(offsets['/api/v1/admin/siakad/kelas']).toEqual([0, 25]);
  await page.getByLabel('Kode mata kuliah').fill('TEST-MK');
  await page.getByLabel('Nama mata kuliah').fill('Created off page');
  await page.route('**/api/v1/admin/siakad/kelas', async (route) => {
    if (route.request().method() !== 'POST') return route.fallback();
    const body = route.request().postDataJSON();
    await route.fulfill({
      status: 201,
      json: {
        data: { ...cls, ...body, id: 'class-created-off-page', version: 1 }
      }
    });
  });
  await page.getByRole('button', { name: 'Buat kelas', exact: true }).click();
  await expect(page.getByLabel('Pilih kelas untuk diedit')).toHaveValue(
    'class-created-off-page'
  );
  await expect(
    page.getByRole('button', { name: 'Simpan kelas', exact: true })
  ).toBeEnabled();
  await page.getByLabel('Nama mata kuliah').fill('Unsaved class edit');
  await page
    .getByRole('button', { name: 'Inventaris & Penyewaan', exact: true })
    .click();
  await page
    .getByRole('button', { name: 'Muat alat lainnya', exact: true })
    .click();
  await expect(
    page.getByLabel('Pilih alat untuk diedit').locator('option')
  ).toHaveCount(28);
  await page.getByLabel('Kode alat', { exact: true }).fill('OFFPAGE-ITEM');
  await page.getByLabel('Nama alat', { exact: true }).fill('Created item');
  await page.route('**/api/v1/admin/siakad/inventaris', async (route) => {
    const body = route.request().postDataJSON();
    await route.fulfill({
      status: 201,
      json: {
        data: { ...item, ...body, id: 'item-created-off-page', version: 1 }
      }
    });
  });
  await page.getByRole('button', { name: 'Buat alat', exact: true }).click();
  await expect(page.getByLabel('Pilih alat untuk diedit')).toHaveValue(
    'item-created-off-page'
  );
  await expect(
    page.getByRole('button', { name: 'Simpan alat', exact: true })
  ).toBeEnabled();
  await page.getByRole('button', { name: 'Kelas', exact: true }).click();
  await expect(page.getByLabel('Nama mata kuliah')).toHaveValue(
    'Unsaved class edit'
  );
});

test('siakad borrower reaches later available items without changing an unresolved rental on history load-more', async ({
  page
}) => {
  await login(page, 'siakad', 'mahasiswa@example.test');
  const item = (await api(page, '/v1/inventory/items')).body.data[0];
  const items = numbered(item, 'borrow-item').map((row, i) => ({
    ...row,
    status: i < 25 ? 'maintenance' : 'available'
  }));
  const rental = {
    id: 'rental-template',
    item_id: item.id,
    item_name: item.name,
    borrower_id: 1,
    borrower_name: 'Mahasiswa Fixture',
    quantity: 1,
    purpose: 'Old rental',
    status: 'requested',
    total_fee: item.rental_fee,
    payment_verified: false,
    requested_at: '2026-11-09T00:00:00Z',
    due_at: '2026-11-12T00:00:00Z',
    scope: item.scope,
    version: 1
  };
  const offsets = await mockPages(page, {
    '/api/v1/inventory/items': items,
    '/api/v1/inventory/rentals': numbered(rental, 'borrow-history')
  });
  await page.goto(`${hosts.siakad}/penyewaan`);
  await page.getByLabel('Keperluan peminjaman').fill('Unresolved retry draft');
  await page
    .getByRole('button', { name: 'Muat alat lainnya', exact: true })
    .click();
  await page.getByLabel('Alat laboratorium').selectOption('borrow-item-26');
  const requests: Array<{ body: string; key: string }> = [];
  await page.route('**/api/v1/inventory/rentals', async (route) => {
    if (route.request().method() !== 'POST') return route.fallback();
    requests.push({
      body: route.request().postData()!,
      key: route.request().headers()['idempotency-key']
    });
    if (requests.length === 1) return route.abort('failed');
    await route.fulfill({
      status: 201,
      json: {
        data: {
          ...rental,
          ...route.request().postDataJSON(),
          id: 'rental-created-off-page'
        }
      }
    });
  });
  await page
    .getByRole('button', { name: 'Ajukan peminjaman', exact: true })
    .click();
  await expect(page.getByRole('alert')).toContainText(
    'Koneksi SIAKAD terputus'
  );
  await page
    .getByRole('button', { name: 'Muat riwayat penyewaan lainnya' })
    .click();
  await expect(page.getByLabel('Alat laboratorium')).toHaveValue(
    'borrow-item-26'
  );
  await expect(page.getByLabel('Keperluan peminjaman')).toHaveValue(
    'Unresolved retry draft'
  );
  await page
    .getByRole('button', { name: 'Ajukan peminjaman', exact: true })
    .click();
  await expect(
    page.locator('[data-rental-id="rental-created-off-page"]')
  ).toBeVisible();
  expect(requests).toHaveLength(2);
  expect(requests[1]).toEqual(requests[0]);
  expect(offsets['/api/v1/inventory/items']).toEqual([0, 25]);
  expect(offsets['/api/v1/inventory/rentals']).toEqual([0, 25]);
});

test('siakad overview and QR attendance expose later classes, schedules, notifications and history', async ({
  page
}) => {
  await login(page, 'siakad', 'mahasiswa@example.test');
  const cls = (await api(page, '/v1/academic/classes')).body.data[0];
  const view = (await api(page, '/v1/academic/schedules')).body.data[0];
  const schedules = Array.from({ length: 27 }, (_, i) => ({
    schedule: { ...view.schedule, id: `student-schedule-${i}` },
    open_session:
      i === 26
        ? {
            id: 'later-open-session',
            schedule_id: 'student-schedule-26',
            class_id: cls.id,
            title: 'Later QR session',
            is_open: true,
            expires_at: '2030-01-01T00:00:00Z',
            scope: cls.scope
          }
        : null
  }));
  const notif = {
    id: 'notification',
    title: 'Later notification',
    message: 'Message',
    created_at: '2026-11-09T00:00:00Z'
  };
  const record = {
    id: 'history',
    session_id: 'old-session',
    class_id: cls.id,
    status: 'hadir',
    validated: false,
    checked_in_at: '2026-11-09T00:00:00Z',
    notes: ''
  };
  await mockPages(page, {
    '/api/v1/academic/classes': numbered(cls, 'student-class'),
    '/api/v1/academic/schedules': schedules,
    '/api/v1/academic/notifications': numbered(notif, 'student-notification'),
    '/api/v1/academic/attendance': numbered(record, 'student-history')
  });
  await page.goto(`${hosts.siakad}/`);
  await page.getByRole('button', { name: 'Muat jadwal lainnya' }).click();
  await expect(
    page.locator('[data-schedule-id="student-schedule-26"]')
  ).toBeVisible();
  await page.getByRole('button', { name: 'Muat kelas lainnya' }).click();
  await expect(
    page.locator('[data-class-id="student-class-26"]')
  ).toBeVisible();
  await page.getByRole('button', { name: 'Muat notifikasi lainnya' }).click();
  await expect(
    page.locator('[data-notification-id="student-notification-26"]')
  ).toBeVisible();
  await page.goto(`${hosts.siakad}/absensi`);
  await page.getByLabel('ID sesi presensi').fill('later-open-session');
  await page.getByLabel('Token QR presensi').fill('unchanged-qr-draft');
  await page
    .getByRole('button', { name: 'Muat sesi presensi lainnya' })
    .click();
  await expect(page.getByLabel('Pilih sesi presensi terbuka')).toHaveValue(
    'later-open-session'
  );
  await expect(page.getByLabel('Token QR presensi')).toHaveValue(
    'unchanged-qr-draft'
  );
  await page
    .getByRole('button', { name: 'Muat riwayat presensi lainnya' })
    .click();
  await expect(
    page.locator('[data-attendance-id="student-history-26"]')
  ).toBeVisible();
});

test('siakad admin reaches later schedules, attendance and rentals and preserves validation replay after refresh', async ({
  page
}) => {
  await login(page, 'admin');
  const cls = (await api(page, '/v1/admin/siakad/kelas')).body.data[0];
  const view = (await api(page, '/v1/admin/siakad/jadwal')).body.data[0];
  const item = (await api(page, '/v1/inventory/items')).body.data[0];
  const schedules = Array.from({ length: 27 }, (_, i) => ({
    schedule: { ...view.schedule, id: `admin-schedule-${i}` },
    open_session: null
  }));
  const records = numbered(
    {
      id: 'attendance',
      session_id: 'session',
      schedule_id: view.schedule.id,
      class_id: cls.id,
      user_id: 1,
      user_name: 'Pagination Student',
      status: 'hadir',
      validated: false,
      notes: '',
      scope: cls.scope,
      version: 1
    },
    'admin-record'
  );
  const rentals = numbered(
    {
      id: 'rental',
      item_id: item.id,
      item_name: item.name,
      borrower_id: 1,
      borrower_name: 'Pagination Borrower',
      quantity: 1,
      purpose: 'Later request',
      status: 'approved',
      total_fee: item.rental_fee,
      payment_verified: false,
      due_at: '2030-01-01T00:00:00Z',
      scope: item.scope,
      version: 1
    },
    'admin-rental'
  );
  const offsets = await mockPages(page, {
    '/api/v1/admin/siakad/jadwal': schedules,
    '/api/v1/admin/siakad/absensi/records': records,
    '/api/v1/admin/siakad/penyewaan': rentals
  });
  await page.goto(`${hosts.admin}/siakad`);
  await page.getByRole('button', { name: 'Muat jadwal lainnya' }).click();
  await expect(
    page.locator('[data-admin-schedule="admin-schedule-26"]')
  ).toBeVisible();
  await page
    .getByLabel('Jadwal sesi presensi')
    .selectOption('admin-schedule-26');
  await page
    .getByLabel('Judul pertemuan')
    .fill('Keep selected off-page schedule');
  await page
    .getByRole('button', { name: 'Muat presensi lainnya', exact: true })
    .click();
  await expect(
    page.locator('[data-admin-attendance="admin-record-26"]')
  ).toBeVisible();
  const edited = page.locator('[data-admin-attendance="admin-record-0"]');
  await edited
    .getByLabel('Catatan validasi Pagination Student')
    .fill('Validation draft');
  const requests: Array<{ body: string; key: string }> = [];
  await page.route(
    '**/api/v1/admin/siakad/absensi/records/admin-record-0',
    async (route) => {
      requests.push({
        body: route.request().postData()!,
        key: route.request().headers()['idempotency-key']
      });
      if (requests.length === 1) {
        records[0] = {
          ...records[0],
          ...route.request().postDataJSON(),
          version: 2
        };
        return route.abort('failed');
      }
      await route.fulfill({ json: { data: records[0] } });
    }
  );
  await edited
    .getByRole('button', { name: 'Validasi presensi Pagination Student' })
    .click();
  await expect(page.getByRole('alert')).toContainText(
    'Koneksi SIAKAD terputus'
  );
  await page
    .getByRole('button', { name: 'Perbarui presensi', exact: true })
    .click();
  await expect(
    edited.getByLabel('Catatan validasi Pagination Student')
  ).toHaveValue('Validation draft');
  await expect(page.getByLabel('Jadwal sesi presensi')).toHaveValue(
    'admin-schedule-26'
  );
  await expect(page.getByLabel('Judul pertemuan')).toHaveValue(
    'Keep selected off-page schedule'
  );
  await edited
    .getByRole('button', { name: 'Validasi presensi Pagination Student' })
    .click();
  await expect(page.locator('.notice[role="status"]')).toContainText(
    'berhasil divalidasi'
  );
  expect(requests).toHaveLength(2);
  expect(requests[1]).toEqual(requests[0]);
  await page
    .getByRole('button', { name: 'Inventaris & Penyewaan', exact: true })
    .click();
  await page.getByRole('button', { name: 'Muat penyewaan lainnya' }).click();
  const later = page.locator('[data-admin-rental="admin-rental-26"]');
  await expect(later).toBeVisible();
  await expect(
    later.getByRole('button', { name: 'Tandai dikembalikan' })
  ).toBeEnabled();
  expect(offsets['/api/v1/admin/siakad/jadwal']).toContain(25);
  expect(offsets['/api/v1/admin/siakad/absensi/records']).toContain(25);
  expect(offsets['/api/v1/admin/siakad/penyewaan']).toContain(25);
});

test('siakad unrelated rental decisions cannot discard an unresolved inventory creation key', async ({
  page
}) => {
  await login(page, 'admin');
  const item = (await api(page, '/v1/inventory/items')).body.data[0];
  const rental = {
    id: 'isolated-rental',
    item_id: item.id,
    item_name: item.name,
    borrower_id: 1,
    borrower_name: 'Isolated Borrower',
    quantity: 1,
    purpose: 'Unrelated request',
    status: 'requested',
    total_fee: item.rental_fee,
    payment_verified: false,
    due_at: '2030-01-01T00:00:00Z',
    scope: item.scope,
    version: 1
  };
  await mockPages(page, { '/api/v1/admin/siakad/penyewaan': [rental] });
  const requests: Array<{ body: string; key: string }> = [];
  await page.route('**/api/v1/admin/siakad/inventaris', async (route) => {
    requests.push({
      body: route.request().postData()!,
      key: route.request().headers()['idempotency-key']
    });
    if (requests.length === 1) return route.abort('failed');
    await route.fulfill({
      status: 201,
      json: {
        data: {
          ...item,
          ...route.request().postDataJSON(),
          id: 'isolated-created-item',
          version: 1
        }
      }
    });
  });
  await page.route(
    '**/api/v1/admin/siakad/penyewaan/isolated-rental',
    async (route) => {
      await route.fulfill({
        json: { data: { ...rental, status: 'approved', version: 2 } }
      });
    }
  );
  await page.goto(`${hosts.admin}/siakad`);
  await page
    .getByRole('button', { name: 'Inventaris & Penyewaan', exact: true })
    .click();
  await page.getByLabel('Kode alat', { exact: true }).fill('UNRESOLVED-CREATE');
  await page.getByLabel('Nama alat', { exact: true }).fill('Keep retry key');
  await page.getByRole('button', { name: 'Buat alat', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText(
    'Koneksi SIAKAD terputus'
  );
  await page
    .getByRole('button', { name: 'Setujui sewa Isolated Borrower' })
    .click();
  await expect(page.locator('.notice[role="status"]')).toContainText(
    'approved'
  );
  await page.getByRole('button', { name: 'Buat alat', exact: true }).click();
  await expect(page.getByLabel('Pilih alat untuk diedit')).toHaveValue(
    'isolated-created-item'
  );
  expect(requests).toHaveLength(2);
  expect(requests[1]).toEqual(requests[0]);
});

test('siakad validation rebases untouched fields but requires persistent explicit same-field conflict resolution', async ({
  page
}) => {
  await login(page, 'admin');
  const cls = (await api(page, '/v1/admin/siakad/kelas')).body.data[0];
  const records = [
    {
      id: 'conflict-record',
      session_id: 'session',
      class_id: cls.id,
      user_id: 1,
      user_name: 'Conflict Student',
      status: 'hadir',
      validated: false,
      notes: 'Initial note',
      scope: cls.scope,
      version: 1
    }
  ];
  await mockPages(page, { '/api/v1/admin/siakad/absensi/records': records });
  await page.goto(`${hosts.admin}/siakad`);
  const article = page.locator('[data-admin-attendance="conflict-record"]');
  const status = article.getByLabel('Status validasi Conflict Student');
  const notes = article.getByLabel('Catatan validasi Conflict Student');
  const validate = article.getByRole('button', {
    name: 'Validasi presensi Conflict Student'
  });
  await notes.fill('My unsaved note');
  records[0] = { ...records[0], status: 'izin', version: 2 };
  await page
    .getByRole('button', { name: 'Perbarui presensi', exact: true })
    .click();
  await expect(status).toHaveValue('izin');
  await expect(notes).toHaveValue('My unsaved note');
  await expect(validate).toBeEnabled();
  records[0] = { ...records[0], notes: 'Another staff note', version: 3 };
  await page
    .getByRole('button', { name: 'Perbarui presensi', exact: true })
    .click();
  await expect(article.getByRole('alert')).toBeVisible();
  await expect(validate).toBeDisabled();
  await expect(notes).toHaveValue('My unsaved note');
  await page
    .getByRole('button', { name: 'Perbarui presensi', exact: true })
    .click();
  await expect(validate).toBeDisabled();
  await expect(article.getByRole('alert')).toBeVisible();
  const writes: Array<{ version: number; status: string; notes: string }> = [];
  await page.route(
    '**/api/v1/admin/siakad/absensi/records/conflict-record',
    async (route) => {
      writes.push(route.request().postDataJSON());
      records[0] = {
        ...records[0],
        ...route.request().postDataJSON(),
        version: 4
      };
      await route.fulfill({ json: { data: records[0] } });
    }
  );
  await article
    .getByRole('button', { name: 'Terapkan perubahan saya', exact: true })
    .click();
  await expect(validate).toBeEnabled();
  expect(writes).toHaveLength(0);
  await validate.click();
  await expect(page.locator('.notice[role="status"]')).toContainText(
    'berhasil divalidasi'
  );
  expect(writes).toHaveLength(1);
  expect(writes[0]).toMatchObject({
    version: 3,
    status: 'izin',
    notes: 'My unsaved note'
  });
  await status.selectOption('hadir');
  records[0] = { ...records[0], status: 'terlambat', version: 5 };
  await page
    .getByRole('button', { name: 'Perbarui presensi', exact: true })
    .click();
  await expect(validate).toBeDisabled();
  await article
    .getByRole('button', { name: 'Gunakan data server', exact: true })
    .click();
  await expect(status).toHaveValue('terlambat');
  await expect(article.getByRole('alert')).toHaveCount(0);
  expect(writes).toHaveLength(1);
});
