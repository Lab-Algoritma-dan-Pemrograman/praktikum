#!/bin/bash
# Gladi resik 005 di replika prod. Dipanggil dari project_ap.
cd "E:/rekapProject/project_ap" || exit 1
C=algohub-r5
BK=backups/pre005_20260913_194140.sql

docker rm -f $C >/dev/null 2>&1
docker run -d --name $C -e POSTGRES_PASSWORD=test -e POSTGRES_DB=algohub postgres:18-alpine >/dev/null
for i in $(seq 1 40); do docker exec $C pg_isready -U postgres >/dev/null 2>&1 && break; sleep 2; done

docker cp "$BK" $C:/prod.sql >/dev/null
docker cp docs/sql/005_elearning.sql $C:/005.sql >/dev/null
docker cp docs/sql/005_rollback.sql  $C:/005r.sql >/dev/null

# restore pakai ON_ERROR_STOP=0: 'schema public already exists' itu jinak
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=0 -f /prod.sql >/tmp/res.log 2>&1
echo "restore: errcount=$(grep -ciE 'ERROR' /tmp/res.log)  (semua jinak jika cuma schema-public)"
grep -iE 'ERROR' /tmp/res.log | head -3
docker exec $C psql -U postgres -d algohub -tAc "select 'tabel='||count(*)||' users='||(select count(*) from users) from information_schema.tables where table_schema='public';"

echo "=== RUN#1 ==="
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /005.sql >/tmp/r1.log 2>&1
echo "run1 exit=$?"; grep -iE 'ERROR' /tmp/r1.log | head -3

echo "=== RUN#2 idempoten ==="
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /005.sql >/tmp/r2.log 2>&1
echo "run2 exit=$?"; grep -iE 'ERROR' /tmp/r2.log | head -3

echo "=== isi dummy, lalu RUN#3: data harus SELAMAT ==="
docker exec $C psql -U postgres -d algohub -tAc "insert into level(id,judul) values ('x','X') on conflict do nothing; insert into modul(id,level_id,judul) values ('xm','x','XM') on conflict do nothing; insert into materi(id,modul_id,judul) values ('xl','xm','XL') on conflict do nothing; select 'materi_sebelum='||count(*) from materi;"
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /005.sql >/tmp/r3.log 2>&1
echo "run3 exit=$?"; grep -iE 'ERROR' /tmp/r3.log | head -3
docker exec $C psql -U postgres -d algohub -tAc "select 'materi_setelah_run3='||count(*) from materi;"

echo "=== view materi_mahasiswa sembunyikan solusi? ==="
docker exec $C psql -U postgres -d algohub -tAc "select 'punya_solusi_di_view='||count(*) from information_schema.columns where table_schema='public' and table_name='materi_mahasiswa' and column_name='solusi';"

echo "=== ROLLBACK ==="
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /005r.sql >/tmp/rb.log 2>&1
echo "rollback exit=$?"; grep -iE 'ERROR' /tmp/rb.log | head -3
docker exec $C psql -U postgres -d algohub -tAc "select 'sisa_tabel_005='||count(*) from information_schema.tables where table_schema='public' and table_name in ('level','modul','profil_belajar','progres_belajar','pencapaian','pencapaian_terbuka','game_soal','game_riwayat','game_konfigurasi','playground_contoh');"
docker exec $C psql -U postgres -d algohub -tAc "select 'materi_kembali_bentuk004='||count(*) from information_schema.columns where table_schema='public' and table_name='materi' and column_name='konten';"

echo "=== RUN#4 pulih setelah rollback ==="
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /005.sql >/tmp/r4.log 2>&1
echo "run4 exit=$?"; grep -iE 'ERROR' /tmp/r4.log | head -3
echo "SELESAI"
