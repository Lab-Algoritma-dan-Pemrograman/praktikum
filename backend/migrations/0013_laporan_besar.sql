-- Laporan besar (final practicum report) core tables. No users, credentials,
-- or domain fixtures are seeded here; payloads stay in the data JSON column.
create table public.academic_laporan_besar (
  id text primary key check (length(id) between 1 and 128),
  parent_id text not null default '', owner_id bigint not null default 0,
  scope jsonb not null check (jsonb_typeof(scope) = 'object'),
  status text not null default '', unique_key text not null default '',
  expires_at timestamptz, data jsonb not null check (jsonb_typeof(data) = 'object')
);
create unique index academic_laporan_besar_unique_key on public.academic_laporan_besar(unique_key) where unique_key <> '';
create index academic_laporan_besar_parent on public.academic_laporan_besar(parent_id,id);
create index academic_laporan_besar_owner on public.academic_laporan_besar(owner_id,id);
create index academic_laporan_besar_due on public.academic_laporan_besar(status,expires_at,id);
create index academic_laporan_besar_scope on public.academic_laporan_besar using gin(scope);
alter table public.academic_laporan_besar enable row level security;
revoke all on public.academic_laporan_besar from public, anon, authenticated;

create table public.academic_laporan_pengumpulan (
  id text primary key check (length(id) between 1 and 128),
  parent_id text not null default '', owner_id bigint not null default 0,
  scope jsonb not null check (jsonb_typeof(scope) = 'object'),
  status text not null default '', unique_key text not null default '',
  expires_at timestamptz, data jsonb not null check (jsonb_typeof(data) = 'object')
);
create unique index academic_laporan_pengumpulan_unique_key on public.academic_laporan_pengumpulan(unique_key) where unique_key <> '';
create index academic_laporan_pengumpulan_parent on public.academic_laporan_pengumpulan(parent_id,id);
create index academic_laporan_pengumpulan_owner on public.academic_laporan_pengumpulan(owner_id,id);
create index academic_laporan_pengumpulan_due on public.academic_laporan_pengumpulan(status,expires_at,id);
create index academic_laporan_pengumpulan_scope on public.academic_laporan_pengumpulan using gin(scope);
alter table public.academic_laporan_pengumpulan enable row level security;
revoke all on public.academic_laporan_pengumpulan from public, anon, authenticated;

alter table public.academic_laporan_besar
  add constraint academic_laporan_besar_state check(status in ('active','inactive'));

alter table public.academic_laporan_pengumpulan
  add constraint academic_laporan_pengumpulan_laporan_fk foreign key(parent_id) references public.academic_laporan_besar(id),
  add constraint academic_laporan_pengumpulan_user_fk foreign key(owner_id) references public.users(id),
  add constraint academic_laporan_pengumpulan_state check(status in ('belum','terkumpul')),
  add constraint academic_laporan_pengumpulan_unique unique(parent_id,owner_id);
