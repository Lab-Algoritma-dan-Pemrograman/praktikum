#!/bin/bash
# Uji alur main sesungguhnya: mulai -> jawab -> selesai, lalu periksa penilaian server.
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
PORT=8097
export PORT APP_ENV=development
H="http://127.0.0.1:$PORT/api"
T="$LOCALAPPDATA/Temp"

go build -o "$T/labap_c.exe" ./cmd/server || exit 1
"$T/labap_c.exe" > "$T/labap_c.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null' EXIT
for i in $(seq 1 40); do curl -s -o /dev/null "$H/health" 2>/dev/null && break; sleep 0.5; done

DB_HOST=$(grep '^DB_HOST=' .env|cut -d= -f2); DB_PORT=$(grep '^DB_PORT=' .env|cut -d= -f2)
DB_USER=$(grep '^DB_USER=' .env|cut -d= -f2); DB_NAME=$(grep '^DB_NAME=' .env|cut -d= -f2)
export PGPASSWORD=$(grep '^DB_PASSWORD=' .env|cut -d= -f2)
# Fungsi, bukan variabel: conninfo harus tetap SATU argumen.
q() { psql "host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=require" -tAc "$1"; }

ROW=$(q "select id||' '||nim from users where role='mahasiswa' order by id limit 1")
UID_M=$(echo "$ROW"|cut -d' ' -f1); NIM_M=$(echo "$ROW"|cut -d' ' -f2)
TOK=$(go run ./cmd/mktoken -uid "$UID_M" -nim "$NIM_M" -role mahasiswa)

echo "### keadaan SEBELUM (mahasiswa id=$UID_M)"
q "select 'riwayat='||count(*) from game_riwayat" 
q "select 'xp='||coalesce(xp,0)||' streak='||coalesce(streak,0) from profil_belajar where user_id=$UID_M"
XP_AWAL=$(q "select coalesce(xp,0) from profil_belajar where user_id=$UID_M")
RIW_AWAL=$(q "select count(*) from game_riwayat")

echo
echo "### 1. mulai: ambil soal (tanpa kunci jawaban)"
# bahasa & jumlah lewat QUERY, sama seperti frontend (handler baca c.Query).
SOAL=$(curl -s -X POST -H "Authorization: Bearer $TOK" "$H/game/mulai?bahasa=c&jumlah=3")
IDS=$(echo "$SOAL" | grep -o '"id":[0-9]*' | cut -d: -f2 | tr '\n' ' ')
echo "  soal id: $IDS"
echo "  bocor baris_bug? $(echo "$SOAL"|grep -c baris_bug) (WAJIB 0)"

# Kunci jawaban sebenarnya, dibaca LANGSUNG dari DB untuk memeriksa penilaian server.
JWB="["; BENAR_ASLI=0; N=0
for id in $IDS; do
  KUNCI=$(q "select baris_bug from game_soal where id=$id")
  [ $N -gt 0 ] && JWB="$JWB,"
  # sengaja: soal pertama dijawab BENAR, sisanya SALAH
  if [ $N -eq 0 ]; then
    JWB="$JWB{\"soal_id\":$id,\"baris_dipilih\":$KUNCI}"; BENAR_ASLI=$((BENAR_ASLI+1))
  else
    SALAH=$(( KUNCI + 1 ))
    JWB="$JWB{\"soal_id\":$id,\"baris_dipilih\":$SALAH}"
  fi
  N=$((N+1))
done
JWB="$JWB]"
echo "  dikirim: $N jawaban, yang benar seharusnya = $BENAR_ASLI"

echo
echo "### 2. selesai: server menilai"
HASIL=$(curl -s -X POST -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
  -d "{\"jawaban\":$JWB}" "$H/game/selesai")
echo "  $HASIL" | head -c 400
echo
BENAR_SRV=$(echo "$HASIL"|grep -o '"benar":[0-9]*'|head -1|cut -d: -f2)
echo "  server bilang benar=$BENAR_SRV, seharusnya=$BENAR_ASLI -> $([ "$BENAR_SRV" = "$BENAR_ASLI" ] && echo COCOK || echo TIDAK COCOK)"

echo
echo "### 3. keadaan SESUDAH"
XP_AKHIR=$(q "select coalesce(xp,0) from profil_belajar where user_id=$UID_M")
RIW_AKHIR=$(q "select count(*) from game_riwayat")
echo "  xp: $XP_AWAL -> $XP_AKHIR (naik $((XP_AKHIR-XP_AWAL)))"
echo "  riwayat: $RIW_AWAL -> $RIW_AKHIR"

echo
echo "### 4. BERSIHKAN jejak uji dari prod"
q "delete from game_riwayat where user_id=$UID_M and dimainkan > now() - interval '5 minutes'"
q "update profil_belajar set xp=$XP_AWAL where user_id=$UID_M"
q "delete from sesi_aktif where user_id=$UID_M"
echo "  riwayat sekarang: $(q 'select count(*) from game_riwayat') (awal $RIW_AWAL)"
echo "  xp sekarang: $(q "select xp from profil_belajar where user_id=$UID_M") (awal $XP_AWAL)"
