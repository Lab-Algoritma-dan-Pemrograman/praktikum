#!/bin/bash
# Uji anti-curang: skor palsu, soal palsu, kuota, dan kebocoran kunci jawaban.
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
PORT=8095
export PORT APP_ENV=development
H="http://127.0.0.1:$PORT/api"
T="$LOCALAPPDATA/Temp"

go build -o "$T/labap_x.exe" ./cmd/server || exit 1
"$T/labap_x.exe" > "$T/labap_x.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null' EXIT
for i in $(seq 1 40); do curl -s -o /dev/null "$H/health" 2>/dev/null && break; sleep 0.5; done

DB_HOST=$(grep '^DB_HOST=' .env|cut -d= -f2); DB_PORT=$(grep '^DB_PORT=' .env|cut -d= -f2)
DB_USER=$(grep '^DB_USER=' .env|cut -d= -f2); DB_NAME=$(grep '^DB_NAME=' .env|cut -d= -f2)
export PGPASSWORD=$(grep '^DB_PASSWORD=' .env|cut -d= -f2)
q() { psql "host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=require" -tAc "$1"; }

ROW=$(q "select id||' '||nim from users where role='mahasiswa' order by id limit 1")
UID_M=$(echo "$ROW"|cut -d' ' -f1); NIM_M=$(echo "$ROW"|cut -d' ' -f2)
TOK=$(go run ./cmd/mktoken -uid "$UID_M" -nim "$NIM_M" -role mahasiswa)

XP_AWAL=$(q "select coalesce(xp,0) from profil_belajar where user_id=$UID_M")
RIW_AWAL=$(q "select count(*) from game_riwayat")
echo "awal: xp=$XP_AWAL riwayat=$RIW_AWAL"

# Ambil satu soal c yang nyata + kunci aslinya.
SID=$(q "select id from game_soal where bahasa='c' order by id limit 1")
KUNCI=$(q "select baris_bug from game_soal where id=$SID")
SALAH=$(( KUNCI + 1 ))
echo "soal uji id=$SID kunci=$KUNCI"

echo
echo "########## A. kirim skor palsu (field xp/skor dipaksa) ##########"
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -d "{\"jawaban\":[{\"soal_id\":$SID,\"baris_dipilih\":$SALAH}],\"xp_didapat\":999999,\"benar\":999,\"skor\":999999}" \
  "$H/game/selesai")
echo "  $RESP" | head -c 300; echo
echo "  mengandung 999999? $(echo "$RESP"|grep -c 999999) (WAJIB 0)"
XP_A=$(q "select coalesce(xp,0) from profil_belajar where user_id=$UID_M")
echo "  xp: $XP_AWAL -> $XP_A (jawaban SALAH, kenaikan WAJIB 0)"

echo
echo "########## B. soal_id palsu (tidak ada di DB) ##########"
RESP=$(curl -s -w " HTTP=%{http_code}" -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -d '{"jawaban":[{"soal_id":999999,"baris_dipilih":1}]}' "$H/game/selesai")
echo "  $RESP" | head -c 300; echo

echo
echo "########## C. jawaban kosong ##########"
RESP=$(curl -s -w " HTTP=%{http_code}" -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -d '{"jawaban":[]}' "$H/game/selesai")
echo "  $RESP" | head -c 300; echo

echo
echo "########## D. kuota mingguan (batas=1) ##########"
ST=$(curl -s -H "Authorization: Bearer $TOK" "$H/game/status")
echo "  status: $ST"
MULAI=$(curl -s -w " HTTP=%{http_code}" -X POST -H "Authorization: Bearer $TOK" "$H/game/mulai?bahasa=c&jumlah=3")
echo "  mulai lagi setelah kuota terpakai: $(echo "$MULAI"|tail -c 60)"

echo
echo "########## E. bahasa python NONAKTIF di prod ##########"
PY=$(curl -s -w " HTTP=%{http_code}" -X POST -H "Authorization: Bearer $TOK" "$H/game/mulai?bahasa=python&jumlah=3")
echo "  $(echo "$PY"|tail -c 80)"

echo
echo "########## F. BERSIHKAN ##########"
q "delete from game_riwayat where user_id=$UID_M and dimainkan > now() - interval '10 minutes'"
q "update profil_belajar set xp=$XP_AWAL where user_id=$UID_M"
q "delete from sesi_aktif where user_id=$UID_M"
echo "  riwayat: $(q 'select count(*) from game_riwayat') (awal $RIW_AWAL)"
echo "  xp: $(q "select xp from profil_belajar where user_id=$UID_M") (awal $XP_AWAL)"
