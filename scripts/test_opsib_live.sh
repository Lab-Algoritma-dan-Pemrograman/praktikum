#!/bin/bash
# Uji live seluruh endpoint yang dipakai frontend elearning setelah Opsi B.
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
PORT=8098
export PORT APP_ENV=development
H="http://127.0.0.1:$PORT/api"
T="$LOCALAPPDATA/Temp"

go build -o "$T/labap_b.exe" ./cmd/server || exit 1
"$T/labap_b.exe" > "$T/labap_b.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null' EXIT

for i in $(seq 1 40); do
  curl -s -o /dev/null "$H/health" 2>/dev/null && break
  sleep 0.5
done

# Ambil satu mahasiswa asli dari prod untuk token uji.
DB_HOST=$(grep '^DB_HOST=' .env|cut -d= -f2); DB_PORT=$(grep '^DB_PORT=' .env|cut -d= -f2)
DB_USER=$(grep '^DB_USER=' .env|cut -d= -f2); DB_NAME=$(grep '^DB_NAME=' .env|cut -d= -f2)
export PGPASSWORD=$(grep '^DB_PASSWORD=' .env|cut -d= -f2)
ROW=$(psql "host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=require" -tAc \
  "select id||' '||nim from users where role='mahasiswa' order by id limit 1")
UID_M=$(echo "$ROW"|cut -d' ' -f1); NIM_M=$(echo "$ROW"|cut -d' ' -f2)
ROWA=$(psql "host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=require" -tAc \
  "select id||' '||nim from users where role in ('asisten','koordinator') order by id limit 1")
UID_A=$(echo "$ROWA"|cut -d' ' -f1); NIM_A=$(echo "$ROWA"|cut -d' ' -f2)

TOK_M=$(go run ./cmd/mktoken -uid "$UID_M" -nim "$NIM_M" -role mahasiswa)
ROLE_A=$(psql "host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=require" -tAc "select role from users where id=$UID_A")
TOK_A=$(go run ./cmd/mktoken -uid "$UID_A" -nim "$NIM_A" -role "$ROLE_A")
echo "mahasiswa uji: id=$UID_M nim=$NIM_M | staf uji: id=$UID_A role=$ROLE_A"
echo

echo "########## 1. tanpa token WAJIB ditolak ##########"
for p in "/belajar/sesi" "/game/status" "/admin/belajar/users" "/admin/game/soal"; do
  code=$(curl -s -o /dev/null -w '%{http_code}' "$H$p")
  [ "$code" = "401" ] && echo "  OK    401  $p" || echo "  BOCOR $code  $p"
done

echo
echo "########## 2. token mahasiswa: endpoint belajar ##########"
for p in "/belajar/sesi" "/belajar/level" "/belajar/modul" "/belajar/materi" "/belajar/progres" "/belajar/pencapaian" "/game/konfigurasi" "/game/status" "/game/peringkat?limit=5"; do
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TOK_M" "$H$p")
  [ "$code" = "200" ] && echo "  OK    200  $p" || echo "  GAGAL $code  $p"
done

echo
echo "########## 3. mahasiswa DILARANG masuk jalur admin ##########"
for p in "/admin/belajar/users" "/admin/game/soal" "/admin/monitoring/sesi"; do
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TOK_M" "$H$p")
  [ "$code" = "403" ] && echo "  OK    403  $p" || echo "  BOCOR $code  $p"
done

echo
echo "########## 4. token admin: jalur admin ##########"
for p in "/admin/belajar/users" "/admin/game/soal" "/admin/monitoring/sesi" "/admin/audit-logs?limit=5"; do
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $TOK_A" "$H$p")
  [ "$code" = "200" ] && echo "  OK    200  $p" || echo "  GAGAL $code  $p"
done

echo
echo "########## 5. KUNCI JAWABAN bocor ke mahasiswa? ##########"
KUR=$(curl -s -H "Authorization: Bearer $TOK_M" "$H/belajar/materi")
echo "  materi: $(echo "$KUR"|wc -c) byte, kemunculan '\"solusi\"': $(echo "$KUR"|grep -o '"solusi"'|wc -l) (WAJIB 0)"
KURA=$(curl -s -H "Authorization: Bearer $TOK_A" "$H/admin/materi")
echo "  materi jalur staf: kemunculan '\"solusi\"': $(echo "$KURA"|grep -o '"solusi"'|wc -l) (boleh >0)"
SOAL=$(curl -s -X POST -H "Authorization: Bearer $TOK_M" -H 'Content-Type: application/json' \
  -d '{"bahasa":"python","jumlah":3}' "$H/game/mulai")
echo "  game/mulai: kemunculan '\"baris_bug\"': $(echo "$SOAL"|grep -o '"baris_bug"'|wc -l) (WAJIB 0)"
echo "  game/mulai: kemunculan '\"penjelasan\"': $(echo "$SOAL"|grep -o '"penjelasan"'|wc -l) (WAJIB 0)"

echo
echo "########## 6. skor palsu ditolak? ##########"
# kirim jawaban asal; server harus menilai sendiri, bukan menerima skor kiriman
HASIL=$(curl -s -X POST -H "Authorization: Bearer $TOK_M" -H 'Content-Type: application/json' \
  -d '{"jawaban":[{"soal_id":1,"baris_dipilih":0}],"benar":999,"xp_didapat":999999}' "$H/game/selesai")
echo "  respons: $(echo "$HASIL"|head -c 200)"
echo "  mengandung xp 999999? $(echo "$HASIL"|grep -c 999999) (WAJIB 0)"

echo
echo "########## 7. log server ##########"
grep -iE "panic|fatal" "$T/labap_b.log" | head -5 || echo "  tidak ada panic"
