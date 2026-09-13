#!/bin/bash
# Gladi resik 006 di replika, lalu terapkan ke prod.
cd "E:/rekapProject/project_ap" || exit 1
C=algohub-r5

echo "########## GLADI RESIK (container) ##########"
docker info >/dev/null 2>&1 || { echo "ABORT: Docker daemon mati. Gladi resik wajib lewat sebelum prod."; exit 1; }
docker rm -f $C >/dev/null 2>&1
docker run -d --name $C -e POSTGRES_PASSWORD=test -e POSTGRES_DB=algohub postgres:18-alpine >/dev/null
for i in $(seq 1 40); do docker exec $C pg_isready -U postgres >/dev/null 2>&1 && break; sleep 2; done
docker cp backups/pre005_20260913_194140.sql $C:/prod.sql >/dev/null
docker cp praktikum/docs/sql/005_elearning.sql $C:/005.sql >/dev/null
docker cp praktikum/docs/sql/006_sesi_aktif.sql $C:/006.sql >/dev/null
docker cp praktikum/docs/sql/006_rollback.sql  $C:/006r.sql >/dev/null
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=0 -f /prod.sql >/dev/null 2>&1
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /005.sql >/dev/null 2>&1

docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /006.sql >/tmp/a.log 2>&1
echo "006 run1 exit=$?"; grep -i error /tmp/a.log | head -3
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /006.sql >/tmp/b.log 2>&1
echo "006 run2 (idempoten) exit=$?"; grep -i error /tmp/b.log | head -3
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /006r.sql >/tmp/c.log 2>&1
echo "rollback exit=$?"; grep -i error /tmp/c.log | head -3
docker exec $C psql -U postgres -d algohub -tAc "select 'sisa_sesi_aktif='||count(*) from information_schema.tables where table_schema='public' and table_name='sesi_aktif';"
docker exec $C psql -U postgres -d algohub -q -v ON_ERROR_STOP=1 -f /006.sql >/tmp/d.log 2>&1
echo "006 run3 setelah rollback exit=$?"

echo "=== uji FK: heartbeat user tak ada harus DITOLAK ==="
docker exec $C psql -U postgres -d algohub -tAc "insert into sesi_aktif(user_id,aktivitas) values (999999,'lesson');" 2>&1 | grep -oE "violates foreign key constraint|ERROR" | head -1
echo "=== uji upsert: dua heartbeat user sama -> tetap 1 baris ==="
docker exec $C psql -U postgres -d algohub -tAc "insert into sesi_aktif(user_id,aktivitas) values (1150,'lesson') on conflict (user_id) do update set aktivitas=excluded.aktivitas, detak_terakhir=now(); insert into sesi_aktif(user_id,aktivitas) values (1150,'game') on conflict (user_id) do update set aktivitas=excluded.aktivitas, detak_terakhir=now(); select 'baris='||count(*)||' aktivitas='||max(aktivitas) from sesi_aktif;"

echo
echo "########## PROD ##########"
cd praktikum/backend || exit 1
DB_HOST=$(grep '^DB_HOST=' .env|cut -d= -f2); DB_PORT=$(grep '^DB_PORT=' .env|cut -d= -f2)
DB_USER=$(grep '^DB_USER=' .env|cut -d= -f2); DB_NAME=$(grep '^DB_NAME=' .env|cut -d= -f2)
export PGPASSWORD=$(grep '^DB_PASSWORD=' .env|cut -d= -f2-)
CONN="host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=require"
cd ../..
psql "$CONN" -q -v ON_ERROR_STOP=1 -f praktikum/docs/sql/006_sesi_aktif.sql
echo "prod 006 exit=$?"
psql "$CONN" -tAc "select 'sesi_aktif ada='||count(*) from information_schema.tables where table_schema='public' and table_name='sesi_aktif';"
echo "SELESAI"
