import assert from 'node:assert/strict';

export async function checkSiakadContract({
  call,
  student,
  admin,
  authOrigin,
  adminOrigin
}) {
  let keyNumber = 0;
  const key = () => `contract-siakad-${++keyNumber}`;
  const request = (cookies, origin, method, template, options = {}) =>
    call(method, template, {
      cookies,
      origin,
      ...options,
      headers: {
        ...(method === 'get'
          ? {}
          : { 'Idempotency-Key': options.key ?? key() }),
        ...options.headers
      }
    });
  const s = (method, template, options) =>
    request(student, authOrigin, method, template, options);
  const a = (method, template, options) =>
    request(admin, adminOrigin, method, template, options);
  const data = (result) => result.body.data;
  const conflict = { status: 409, errorCode: 'CONFLICT' };
  const forbidden = { status: 403, errorCode: 'FORBIDDEN' };

  const loginAs = async (email, origin = adminOrigin) =>
    (
      await call('post', '/v1/auth/password', {
        origin,
        body: { email, password: 'fixture-password' }
      })
    ).cookies;

  const sekretaris = await loginAs('sekretaris@example.test');
  const k3 = await loginAs('k3@example.test');
  const bendahara = await loginAs('bendahara@example.test');
  const peminjam = await loginAs('peminjam@example.test', authOrigin);

  const sek = (method, template, options) =>
    request(sekretaris, adminOrigin, method, template, options);
  const k3Req = (method, template, options) =>
    request(k3, adminOrigin, method, template, options);
  const ben = (method, template, options) =>
    request(bendahara, adminOrigin, method, template, options);
  const bor = (method, template, options) =>
    request(peminjam, authOrigin, method, template, options);

  await request([], authOrigin, 'get', '/v1/academic/classes', {
    status: 401,
    errorCode: 'UNAUTHENTICATED'
  });

  const studentClasses = data(await s('get', '/v1/academic/classes'));
  assert.equal(studentClasses.length, 1);
  assert.equal(studentClasses[0].id, 'class-101');
  assert.ok(!('student_ids' in studentClasses[0]));
  assert.ok(!('assistant_ids' in studentClasses[0]));

  const studentSchedules = data(await s('get', '/v1/academic/schedules'));
  assert.equal(studentSchedules.length, 1);
  assert.equal(studentSchedules[0].schedule.id, 'schedule-1');
  assert.equal(studentSchedules[0].open_session, null);

  // Division boundaries: sekretaris cannot manage classes or inventory; k3 cannot write schedules.
  const scope = { domain: 'siakad', institution: 'lab' };
  const classInput = {
    code: 'IF-102-B',
    course_code: 'IF1102',
    course_name: 'Struktur Data',
    period_name: 'Ganjil 2026/2027',
    shift: 2,
    student_ids: [1],
    assistant_ids: [3],
    scope
  };
  await sek('post', '/v1/admin/siakad/kelas', {
    body: classInput,
    ...forbidden
  });

  const classKey = key();
  const createdClass = data(
    await a('post', '/v1/admin/siakad/kelas', {
      body: classInput,
      key: classKey,
      status: 201
    })
  );
  assert.equal(
    data(
      await a('post', '/v1/admin/siakad/kelas', {
        body: classInput,
        key: classKey,
        status: 201
      })
    ).id,
    createdClass.id
  );
  const classUpdate = {
    version: createdClass.version,
    code: 'IF-102-B',
    course_code: 'IF1102',
    course_name: 'Struktur Data Lanjut',
    period_name: 'Ganjil 2026/2027',
    shift: 2,
    student_ids: [1],
    assistant_ids: [3],
    active: true
  };
  assert.equal(
    data(
      await a('patch', '/v1/admin/siakad/kelas/{id}', {
        path: `/v1/admin/siakad/kelas/${createdClass.id}`,
        body: classUpdate
      })
    ).version,
    2
  );
  await a('patch', '/v1/admin/siakad/kelas/{id}', {
    path: `/v1/admin/siakad/kelas/${createdClass.id}`,
    body: classUpdate,
    ...conflict
  });
  const adminClasses = data(await a('get', '/v1/admin/siakad/kelas'));
  assert.ok(adminClasses.length >= 2);

  // Schedule creation by sekretaris (allowed) vs k3 (forbidden).
  const scheduleInput = {
    class_id: createdClass.id,
    day_of_week: 'Rabu',
    shift: 2,
    room: 'Lab Lanjut 2',
    starts_at: '13:00 WIB',
    ends_at: '15:00 WIB'
  };
  await k3Req('post', '/v1/admin/siakad/jadwal', {
    body: scheduleInput,
    ...forbidden
  });
  const createdSchedule = data(
    await sek('post', '/v1/admin/siakad/jadwal', {
      body: scheduleInput,
      status: 201
    })
  );
  assert.equal(createdSchedule.class_id, createdClass.id);
  const adminSchedules = data(await sek('get', '/v1/admin/siakad/jadwal'));
  assert.ok(adminSchedules.length >= 2);

  // QR Attendance session open, one-time token, student check-in, validation, close.
  const openKey = key();
  const openInput = {
    schedule_id: 'schedule-1',
    title: 'Pertemuan 1 — Pengenalan C',
    duration_minutes: 30
  };
  const opened = data(
    await sek('post', '/v1/admin/siakad/absensi/sesi', {
      body: openInput,
      key: openKey,
      status: 201
    })
  );
  assert.ok(opened.qr_token);
  const replayOpen = data(
    await sek('post', '/v1/admin/siakad/absensi/sesi', {
      body: openInput,
      key: openKey,
      status: 201
    })
  );
  assert.equal(replayOpen.id, opened.id);
  assert.ok(!('qr_token' in replayOpen));

  await s('post', '/v1/academic/attendance', {
    body: { session_id: opened.id, qr_token: 'invalid-token' },
    ...forbidden
  });
  await bor('post', '/v1/academic/attendance', {
    body: { session_id: opened.id, qr_token: opened.qr_token },
    status: 404,
    errorCode: 'NOT_FOUND'
  });

  const checkinKey = key();
  const record = data(
    await s('post', '/v1/academic/attendance', {
      body: { session_id: opened.id, qr_token: opened.qr_token },
      key: checkinKey
    })
  );
  assert.equal(record.status, 'hadir');
  assert.equal(record.verified_by_qr, true);
  assert.equal(record.validated, false);
  // Repeating check-in with a new key is idempotent for the same student and session.
  const repeatRecord = data(
    await s('post', '/v1/academic/attendance', {
      body: { session_id: opened.id, qr_token: opened.qr_token }
    })
  );
  assert.equal(repeatRecord.id, record.id);

  const studentAttList = data(await s('get', '/v1/academic/attendance'));
  assert.equal(studentAttList.length, 1);

  const adminAttRecords = data(
    await sek('get', '/v1/admin/siakad/absensi/records', {
      path: `/v1/admin/siakad/absensi/records?session_id=${opened.id}`
    })
  );
  assert.equal(adminAttRecords.length, 1);

  const validated = data(
    await sek('patch', '/v1/admin/siakad/absensi/records/{id}', {
      path: `/v1/admin/siakad/absensi/records/${record.id}`,
      body: {
        version: record.version,
        status: 'hadir',
        validated: true,
        notes: 'Hadir tepat waktu'
      }
    })
  );
  assert.equal(validated.validated, true);
  assert.equal(validated.version, 2);

  const closedSession = data(
    await sek('post', '/v1/admin/siakad/absensi/sesi/{id}/close', {
      path: `/v1/admin/siakad/absensi/sesi/${opened.id}/close`,
      body: {}
    })
  );
  assert.equal(closedSession.is_open, false);
  for (const replayKey of [checkinKey, key()]) {
    await s('post', '/v1/academic/attendance', {
      body: { session_id: opened.id, qr_token: opened.qr_token },
      key: replayKey,
      status: 409,
      errorCode: 'ATTENDANCE_CLOSED'
    });
  }

  // Inventory & Rental lifecycle across k3, bendahara, and peminjam.
  const itemInput = {
    code: 'INV-RPI-01',
    name: 'Raspberry Pi 5 Kit',
    category: 'Single Board Computer',
    total_quantity: 2,
    available_quantity: 2,
    rental_fee: 40000,
    status: 'available',
    scope
  };
  await sek('post', '/v1/admin/siakad/inventaris', {
    body: itemInput,
    ...forbidden
  });
  await ben('post', '/v1/admin/siakad/inventaris', {
    body: itemInput,
    ...forbidden
  });
  const createdItem = data(
    await k3Req('post', '/v1/admin/siakad/inventaris', {
      body: itemInput,
      status: 201
    })
  );
  const updatedItem = data(
    await k3Req('patch', '/v1/admin/siakad/inventaris/{id}', {
      path: `/v1/admin/siakad/inventaris/${createdItem.id}`,
      body: {
        version: createdItem.version,
        name: 'Raspberry Pi 5 8GB Kit',
        category: 'Single Board Computer',
        total_quantity: 2,
        available_quantity: 2,
        rental_fee: 40000,
        status: 'available'
      }
    })
  );
  assert.equal(updatedItem.version, 2);

  const items = data(await bor('get', '/v1/inventory/items'));
  assert.ok(items.some((i) => i.id === createdItem.id));

  const dueAt = new Date(Date.now() + 3 * 24 * 3600 * 1000).toISOString();
  await bor('post', '/v1/inventory/rentals', {
    body: {
      item_id: createdItem.id,
      quantity: 5,
      purpose: 'Melebihi stok',
      due_at: dueAt
    },
    status: 409,
    errorCode: 'INSUFFICIENT_STOCK'
  });

  const rental1 = data(
    await bor('post', '/v1/inventory/rentals', {
      body: {
        item_id: createdItem.id,
        quantity: 2,
        purpose: 'Penelitian Tugas Akhir',
        due_at: dueAt
      },
      status: 201
    })
  );
  assert.equal(rental1.status, 'requested');
  assert.equal(rental1.total_fee, 80000);
  assert.equal(rental1.payment_verified, false);

  const rental2 = data(
    await s('post', '/v1/inventory/rentals', {
      body: {
        item_id: createdItem.id,
        quantity: 1,
        purpose: 'Proyek Kelas',
        due_at: dueAt
      },
      status: 201
    })
  );

  assert.equal(data(await bor('get', '/v1/inventory/rentals')).length, 1);
  assert.ok(
    data(await k3Req('get', '/v1/admin/siakad/penyewaan')).length >= 2
  );

  // Bendahara cannot approve rental; K3 cannot verify payment.
  await ben('patch', '/v1/admin/siakad/penyewaan/{id}', {
    path: `/v1/admin/siakad/penyewaan/${rental1.id}`,
    body: { version: rental1.version, action: 'approve' },
    ...forbidden
  });
  await k3Req('patch', '/v1/admin/siakad/penyewaan/{id}', {
    path: `/v1/admin/siakad/penyewaan/${rental1.id}`,
    body: { version: rental1.version, action: 'verify_payment' },
    ...forbidden
  });

  // Finance cannot verify a request before approval.
  await ben('patch', '/v1/admin/siakad/penyewaan/{id}', {
    path: `/v1/admin/siakad/penyewaan/${rental1.id}`,
    body: { version: rental1.version, action: 'verify_payment' },
    ...conflict
  });

  // K3 approves rental1 (consumes all 2 units of createdItem).
  const approved1 = data(
    await k3Req('patch', '/v1/admin/siakad/penyewaan/{id}', {
      path: `/v1/admin/siakad/penyewaan/${rental1.id}`,
      body: { version: rental1.version, action: 'approve' }
    })
  );
  assert.equal(approved1.status, 'approved');

  // Approving rental2 while stock is 0 returns 409 INSUFFICIENT_STOCK.
  await k3Req('patch', '/v1/admin/siakad/penyewaan/{id}', {
    path: `/v1/admin/siakad/penyewaan/${rental2.id}`,
    body: { version: rental2.version, action: 'approve' },
    status: 409,
    errorCode: 'INSUFFICIENT_STOCK'
  });

  // Bendahara verifies payment on rental1.
  const paid1 = data(
    await ben('patch', '/v1/admin/siakad/penyewaan/{id}', {
      path: `/v1/admin/siakad/penyewaan/${rental1.id}`,
      body: { version: approved1.version, action: 'verify_payment' }
    })
  );
  assert.equal(paid1.payment_verified, true);

  // K3 marks rental1 returned (restores 2 units); re-returning is rejected.
  const returned1 = data(
    await k3Req('patch', '/v1/admin/siakad/penyewaan/{id}', {
      path: `/v1/admin/siakad/penyewaan/${rental1.id}`,
      body: { version: paid1.version, action: 'return' }
    })
  );
  assert.equal(returned1.status, 'returned');
  await k3Req('patch', '/v1/admin/siakad/penyewaan/{id}', {
    path: `/v1/admin/siakad/penyewaan/${rental1.id}`,
    body: { version: returned1.version, action: 'return' },
    ...conflict
  });

  const rejected2 = data(await k3Req('patch', '/v1/admin/siakad/penyewaan/{id}', {
    path: `/v1/admin/siakad/penyewaan/${rental2.id}`,
    body: { version: rental2.version, action: 'reject' }
  }));
  await ben('patch', '/v1/admin/siakad/penyewaan/{id}', {
    path: `/v1/admin/siakad/penyewaan/${rejected2.id}`,
    body: { version: rejected2.version, action: 'verify_payment' },
    ...conflict
  });

  const highFeeItem = data(await k3Req('post', '/v1/admin/siakad/inventaris', {
    body: { ...itemInput, code: 'INV-MAX-FEE', total_quantity: 1000, available_quantity: 1000, rental_fee: 100000000 },
    status: 201
  }));
  const highFeeRental = data(await bor('post', '/v1/inventory/rentals', {
    body: { item_id: highFeeItem.id, quantity: 1000, purpose: 'Maximum permitted total fee', due_at: dueAt },
    status: 201
  }));
  assert.equal(highFeeRental.total_fee, 100000000000);

  const studentNotifs = data(await s('get', '/v1/academic/notifications'));
  assert.ok(studentNotifs.length >= 1);
  const borrowerNotifs = data(await bor('get', '/v1/academic/notifications'));
  assert.ok(borrowerNotifs.length >= 2);

  const report = data(await a('get', '/v1/admin/siakad/laporan'));
  assert.ok(report.classes_count >= 2);
  assert.equal(report.total_verified_revenue, 80000);
  assert.match(report.csv_export, /total_verified_revenue,80000/);
  const financeReport = data(await ben('get', '/v1/admin/siakad/laporan'));
  assert.equal(financeReport.total_verified_revenue, 80000);
  for (const actor of [sek, k3Req]) {
    const restrictedReport = data(await actor('get', '/v1/admin/siakad/laporan'));
    assert.equal(restrictedReport.total_verified_revenue, 0);
    assert.match(restrictedReport.csv_export, /total_verified_revenue,0/);
  }

  const audit = data(
    await call('get', '/v1/admin/auditlog', {
      cookies: admin,
      origin: adminOrigin,
      path: '/v1/admin/auditlog?limit=100'
    })
  );
  assert.ok(
    audit.some(
      (event) =>
        event.action.startsWith('academic.') ||
        event.action.startsWith('inventory.')
    )
  );
  assert.ok(
    !JSON.stringify(audit).includes(opened.qr_token),
    'QR token leaked into shared audit log'
  );
}
