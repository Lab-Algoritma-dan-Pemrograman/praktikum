-- Lossless legacy preservation. Private schema: no access for anon/authenticated,
-- service role only. Nothing here is exposed through the application API.
create schema if not exists legacy_preserve;

create table if not exists legacy_preserve.rows (
  source_table text not null,
  source_pk text not null,
  row_data jsonb not null,
  row_sha256 text not null,
  batch_id text not null,
  primary key (source_table, source_pk)
);

create table if not exists legacy_preserve.objects (
  bucket text not null,
  path text not null,
  size bigint not null,
  sha256 text not null,
  mimetype text,
  is_public boolean not null,
  source_meta jsonb,
  primary key (bucket, path)
);

create table if not exists legacy_preserve.batches (
  batch_id text primary key,
  source_fingerprint text not null,
  target_fingerprint text not null,
  counts jsonb not null,
  status text not null,
  created_at timestamptz not null default now(),
  applied_at timestamptz,
  constraint legacy_preserve_batch_status check (status in ('planned', 'applied', 'failed'))
);

create index if not exists legacy_preserve_rows_table_idx on legacy_preserve.rows (source_table);

do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on schema legacy_preserve from anon;
    revoke all on all tables in schema legacy_preserve from anon;
  end if;
  if exists (select 1 from pg_roles where rolname = 'authenticated') then
    revoke all on schema legacy_preserve from authenticated;
    revoke all on all tables in schema legacy_preserve from authenticated;
  end if;
end
$$;
