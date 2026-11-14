# Role, Division, Permission, dan Scope Matrix

**Status:** Target baseline berdasarkan audit project lama

## 1. Keputusan identitas

Role identity hanya empat:

| Role kanonik | Arti |
|---|---|
| `koordinator` | pengelola dan pengambil keputusan lintas domain |
| `asisten` | staf operasional; akun ini juga dipakai untuk console admin |
| `mahasiswa` | peserta pembelajaran/praktikum |
| `peminjam` | pengguna tanpa NIM untuk penyewaan alat |

`admin`, `kordas`, `superadmin`, `praktikan`, `user`, dan `penyewa` adalah alias
legacy yang hanya dipakai pada adapter migrasi. Tidak boleh menjadi role target.

## 2. Divisi asisten

Audit menemukan penggunaan nama divisi berikut di source/migration lama:

```text
sekretaris, bendahara, k3, labops, pendidikan, pdd
```

Divisi adalah assignment/capability, bukan role identity. `asisten` boleh memiliki
satu atau lebih assignment divisi dan scope kelas/shift/sesi. Nama divisi tidak
boleh menjadi satu-satunya pemeriksa authorization; backend memeriksa permission
yang diberikan kepada assignment tersebut.

`bendahara` dipertahankan sebagai divisi target karena sudah muncul pada data dan
migrasi legacy, meskipun seed produksi harus divalidasi sebelum migrasi final.

## 3. Permission target awal

| Permission | Domain | Default |
|---|---|---|
| `user.read`, `user.write`, `user.deactivate` | semua | koordinator; asisten sesuai scope |
| `schedule.read`, `schedule.write`, `schedule.approve` | siakad/praktikum | koordinator; sekretaris sesuai scope |
| `class.manage`, `assistant.assign` | siakad/praktikum | koordinator; asisten sesuai scope |
| `attendance.read`, `attendance.validate`, `attendance.export` | siakad | sesuai scope; sekretaris/K3 dapat diberi explicit grant |
| `inventory.read`, `inventory.manage`, `rental.approve` | siakad | koordinator; K3/labops sesuai grant |
| `finance.read`, `finance.manage` | siakad | koordinator; bendahara sesuai grant |
| `learning.content.read` | e-learning | mahasiswa/asisten |
| `learning.content.manage` | e-learning | koordinator; pendidikan sesuai grant |
| `learning.monitor` | e-learning | koordinator; pendidikan/asisten sesuai scope |
| `practicum.session.manage`, `question.manage` | praktikum | koordinator; asisten sesuai scope |
| `practicum.grade.write`, `grade.export` | praktikum | koordinator; asisten pengampu |
| `audit.read` | semua | koordinator; assignment yang diberi grant |
| `settings.manage` | semua | koordinator |

Daftar ini adalah seed awal. Permission yang belum terbukti dari audit tidak boleh
diam-diam diberikan luas; tambahkan melalui decision log dan test authorization.

## 4. Model data authorization

Gunakan konsep berikut pada ERD target:

```text
users
  └─ user_roles(user_id, role_code, scope)
       └─ role_assignments/user_division_assignments
            └─ permissions
```

`scope` minimal mendukung `domain`, `institution`, `division`, `class_id`,
`shift`, dan `session_id`. Permission check berbentuk:

```text
authenticated user
→ active role assignment
→ permission
→ resource scope/ownership
→ action
```

## 5. Guardrail penting

- Host `admin.algohub.web.id` bukan authorization boundary.
- Frontend menu hanya convenience; Go usecase adalah enforcement utama.
- `koordinator` tidak boleh menjadi bypass tersembunyi di repository; bypass harus
  tercatat sebagai policy yang diuji.
- Pergantian divisi harus diaudit dan tidak boleh mengubah histori audit lama.
- Setiap kombinasi allow/deny minimal memiliki unit test dan API integration test.
