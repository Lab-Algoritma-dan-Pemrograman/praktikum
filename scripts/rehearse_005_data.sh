#!/bin/bash
# Gladi resik PINDAH DATA 005 di replika prod (container), bukan prod.
cd "E:/rekapProject/project_ap" || exit 1
C=algohub-r5
BK=backups/pre005_20260913_194140.sql

docker rm -f $C >/dev/null 2>&1
docker run -d --name $C -p 55433:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=algohub postgres:18-alpine >/dev/null
for i in $(seq 1 40); do docker exec $C pg_isready -U postgres >/dev/null 2>&1 && break; sleep 2; done

docker cp "$BK" $C:/prod.sql >/dev/null
docker cp docs/sql/005_elearning.sql $C:/005.sql >/dev/null
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=0 -f /prod.sql >/tmp/res.log 2>&1
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /005.sql >/tmp/r1.log 2>&1
echo "setup: restore+005 exit=$?"
docker exec $C psql -U postgres -d algohub -tAc "select 'users='||count(*) from public.users;"

DSN="host=127.0.0.1 port=55433 user=postgres password=test dbname=algohub"

echo "=== DRY RUN ==="
python scripts/migrate_005_data.py --dsn "$DSN" --dry-run
echo "dryrun exit=$?"

echo "=== REAL RUN #1 ==="
python scripts/migrate_005_data.py --dsn "$DSN"
echo "run1 exit=$?"

echo "=== REAL RUN #2 (idempoten: jumlah harus SAMA) ==="
python scripts/migrate_005_data.py --dsn "$DSN"
echo "run2 exit=$?"

echo "=== VERIFIKASI JUMLAH BARIS ==="
docker exec $C psql -U postgres -d algohub -c "select 'users' t,count(*) n from users union all select 'level',count(*) from level union all select 'modul',count(*) from modul union all select 'materi',count(*) from materi union all select 'profil_belajar',count(*) from profil_belajar union all select 'progres_belajar',count(*) from progres_belajar union all select 'pencapaian',count(*) from pencapaian union all select 'pencapaian_terbuka',count(*) from pencapaian_terbuka union all select 'game_soal',count(*) from game_soal union all select 'game_riwayat',count(*) from game_riwayat union all select 'game_konfigurasi',count(*) from game_konfigurasi union all select 'playground_contoh',count(*) from playground_contoh order by 1;"

echo "=== CEK PETA NIM Faqod -> 202314020 ==="
docker exec $C psql -U postgres -d algohub -c "select u.id,u.nim,u.nama,pb.xp,pb.level_angka from users u join profil_belajar pb on pb.user_id=u.id where u.nim='202314020';"

echo "=== CEK 4 MAHASISWA BARU ==="
docker exec $C psql -U postgres -d algohub -c "select id,nim,nama,role,email from users where nim in ('202511011','202515064','202515065','202511065') order by id;"

echo "=== CEK view materi_mahasiswa (solusi harus hilang) ==="
docker exec $C psql -U postgres -d algohub -tAc "select 'kolom_view='||count(*) from information_schema.columns where table_schema='public' and table_name='materi_mahasiswa'; select 'ada_solusi='||count(*) from information_schema.columns where table_schema='public' and table_name='materi_mahasiswa' and column_name='solusi';"

echo "=== SAMPEL materi (jsonb utuh?) ==="
docker exec $C psql -U postgres -d algohub -tAc "select id||' | kuis='||(kuis is not null)||' | kasus_uji='||jsonb_array_length(coalesce(kasus_uji,'[]'::jsonb))||' | xp='||xp_hadiah from materi order by id limit 3;"
echo "SELESAI"
