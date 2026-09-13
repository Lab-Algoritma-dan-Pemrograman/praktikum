-- AlgoHub 004: tabel domain yang belum ada di praktikum DB.
-- Praktikum sudah punya: course(asesmen), soal, jawaban_mahasiswa,
-- pengerjaan_course, sesi_praktikum, jadwal, kelas, pedoman_laporan.
-- Yang HILANG dan dibutuhkan 3 app:
--   materi      -> bahan belajar elearning (lessons)
--   absensi     -> kehadiran, fitur harian siakad
--   inventaris  -> barang lab
--   peminjaman  -> dukung role 'peminjam' dari 002 (sekarang role tanpa tabel)
-- Idempoten: aman dijalankan berulang.

begin;

-- 1) MATERI (elearning)
create table if not exists public.materi (
  id          serial primary key,
  sesi_id     integer references public.sesi_praktikum(id) on delete set null,
  judul       varchar(200) not null,
  konten      text,
  urutan      integer not null default 0,
  is_publik   boolean not null default false,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now()
);
create index if not exists idx_materi_sesi on public.materi (sesi_id, urutan);

-- 2) ABSENSI (siakad)
create table if not exists public.absensi (
  id            serial primary key,
  user_id       integer not null references public.users(id) on delete cascade,
  sesi_id       integer references public.sesi_praktikum(id) on delete set null,
  kelas_id      integer references public.kelas(id) on delete set null,
  status        varchar(12) not null default 'hadir',
  waktu         timestamptz not null default now(),
  dicatat_oleh  integer references public.users(id) on delete set null,
  catatan       varchar(255),
  constraint absensi_status_check check (status in ('hadir','izin','sakit','alfa'))
);
create unique index if not exists uq_absensi_user_sesi
  on public.absensi (user_id, sesi_id) where sesi_id is not null;
create index if not exists idx_absensi_kelas on public.absensi (kelas_id, waktu);

-- 3) INVENTARIS (barang lab)
create table if not exists public.inventaris (
  id              serial primary key,
  kode            varchar(50) not null unique,
  nama            varchar(150) not null,
  jumlah_total    integer not null default 1 check (jumlah_total >= 0),
  jumlah_tersedia integer not null default 1 check (jumlah_tersedia >= 0),
  kondisi         varchar(20) not null default 'baik',
  created_at      timestamptz not null default now(),
  constraint inventaris_tersedia_wajar check (jumlah_tersedia <= jumlah_total)
);

-- 4) PEMINJAMAN (dukung role 'peminjam')
create table if not exists public.peminjaman (
  id             serial primary key,
  inventaris_id  integer not null references public.inventaris(id) on delete restrict,
  peminjam_id    integer not null references public.users(id) on delete cascade,
  jumlah         integer not null default 1 check (jumlah > 0),
  status         varchar(12) not null default 'diajukan',
  tgl_pinjam     timestamptz not null default now(),
  tgl_kembali    timestamptz,
  disetujui_oleh integer references public.users(id) on delete set null,
  catatan        varchar(255),
  constraint peminjaman_status_check check (status in ('diajukan','dipinjam','kembali','ditolak'))
);
create index if not exists idx_peminjaman_peminjam on public.peminjaman (peminjam_id, status);
create index if not exists idx_peminjaman_barang on public.peminjaman (inventaris_id, status);

commit;

-- verifikasi
select 'materi' t, count(*) n from public.materi
union all select 'absensi', count(*) from public.absensi
union all select 'inventaris', count(*) from public.inventaris
union all select 'peminjaman', count(*) from public.peminjaman;
