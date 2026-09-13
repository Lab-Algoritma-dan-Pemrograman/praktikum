#!/bin/bash
# Terapkan 005 + pindah data elearning ke PROD praktikum DB.
# Backup sudah diambil: backups/pre005_20260913_194140.sql
set -e
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
DB_HOST=$(grep '^DB_HOST=' .env | cut -d= -f2)
DB_PORT=$(grep '^DB_PORT=' .env | cut -d= -f2)
DB_USER=$(grep '^DB_USER=' .env | cut -d= -f2)
DB_NAME=$(grep '^DB_NAME=' .env | cut -d= -f2)
export PGPASSWORD=$(grep '^DB_PASSWORD=' .env | cut -d= -f2-)
CONN="host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=require"
DSN="host=$DB_HOST port=$DB_PORT user=$DB_USER password=$PGPASSWORD dbname=$DB_NAME sslmode=require"

cd "E:/rekapProject/project_ap"

echo "=== SEBELUM ==="
psql "$CONN" -tAc "select 'users='||count(*) from public.users;"

echo "=== JALANKAN 005 ==="
psql "$CONN" -q -v ON_ERROR_STOP=1 -f docs/sql/005_elearning.sql
echo "005 exit=$?"

echo "=== DRY RUN pindah data ==="
python scripts/migrate_005_data.py --dsn "$DSN" --dry-run

echo "=== PINDAH DATA (COMMIT) ==="
python scripts/migrate_005_data.py --dsn "$DSN"

echo "=== VERIFIKASI PROD ==="
psql "$CONN" -c "select 'users' t,count(*) n from users union all select 'level',count(*) from level union all select 'modul',count(*) from modul union all select 'materi',count(*) from materi union all select 'profil_belajar',count(*) from profil_belajar union all select 'progres_belajar',count(*) from progres_belajar union all select 'pencapaian',count(*) from pencapaian union all select 'pencapaian_terbuka',count(*) from pencapaian_terbuka union all select 'game_soal',count(*) from game_soal union all select 'game_riwayat',count(*) from game_riwayat union all select 'game_konfigurasi',count(*) from game_konfigurasi union all select 'playground_contoh',count(*) from playground_contoh order by 1;"

echo "=== CEK Faqod + 4 mahasiswa baru ==="
psql "$CONN" -c "select u.id,u.nim,u.nama,u.role,pb.xp from users u left join profil_belajar pb on pb.user_id=u.id where u.nim in ('202314020','202511011','202515064','202515065','202511065') order by u.id;"
echo "SELESAI"
