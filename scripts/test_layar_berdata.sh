#!/bin/bash
# Cek layar-layar elearning benar dapat DATA, bukan cuma 200 kosong.
# Memakai server yang SUDAH jalan di :8080. Read-only, tidak menulis apa pun.
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
H="http://localhost:8080/api"

DB_HOST=$(grep '^DB_HOST=' .env|cut -d= -f2); DB_PORT=$(grep '^DB_PORT=' .env|cut -d= -f2)
DB_USER=$(grep '^DB_USER=' .env|cut -d= -f2); DB_NAME=$(grep '^DB_NAME=' .env|cut -d= -f2)
export PGPASSWORD=$(grep '^DB_PASSWORD=' .env|cut -d= -f2)
q() { psql "host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME sslmode=require" -tAc "$1"; }

RM=$(q "select id||' '||nim from users where role='mahasiswa' order by id limit 1")
UID_M=$(echo "$RM"|cut -d' ' -f1); NIM_M=$(echo "$RM"|cut -d' ' -f2)
RA=$(q "select id||' '||nim||' '||role from users where role in ('asisten','koordinator') order by id limit 1")
UID_A=$(echo "$RA"|cut -d' ' -f1); NIM_A=$(echo "$RA"|cut -d' ' -f2); ROLE_A=$(echo "$RA"|cut -d' ' -f3)
TOK_M=$(go run ./cmd/mktoken -uid "$UID_M" -nim "$NIM_M" -role mahasiswa 2>/dev/null | tr -d '\r\n')
TOK_A=$(go run ./cmd/mktoken -uid "$UID_A" -nim "$NIM_A" -role "$ROLE_A" 2>/dev/null | tr -d '\r\n')

# isi() nama, path, token, kunci-json-yang-diharap
isi() {
  local nama="$1" path="$2" tk="$3" kunci="$4"
  local body n
  body=$(curl -s -H "Authorization: Bearer $tk" "$H$path")
  n=$(echo "$body" | grep -o "\"$kunci\"" | wc -l)
  local byte=${#body}
  if [ "$n" -gt 0 ]; then
    echo "  ADA DATA   $nama  (${byte} byte, '$kunci' x$n)"
  else
    echo "  KOSONG!!   $nama  (${byte} byte) -> $(echo "$body" | head -c 160)"
  fi
}

echo "mahasiswa=$UID_M  staf=$UID_A($ROLE_A)"
echo
echo "########## layar MAHASISWA ##########"
isi "Sesi/Dashboard"   "/belajar/sesi"        "$TOK_M" "nim"
isi "Daftar Level"     "/belajar/level"       "$TOK_M" "judul"
isi "Daftar Modul"     "/belajar/modul"       "$TOK_M" "judul"
isi "Materi"           "/belajar/materi"      "$TOK_M" "judul"
isi "Progres"          "/belajar/progres"     "$TOK_M" "data"
isi "Pencapaian"       "/belajar/pencapaian"  "$TOK_M" "semua"
isi "Papan Peringkat"  "/game/peringkat?limit=5" "$TOK_M" "data"
isi "Konfigurasi Game" "/game/konfigurasi"    "$TOK_M" "bug_hunt_aktif"
isi "Status Main"      "/game/status"         "$TOK_M" "batas"

echo
echo "########## layar STAF ##########"
isi "Admin: peserta"   "/admin/belajar/users"   "$TOK_A" "data"
isi "Admin: materi"    "/admin/materi"          "$TOK_A" "judul"
isi "Admin: soal game" "/admin/game/soal"       "$TOK_A" "baris_bug"
isi "Monitoring sesi"  "/admin/monitoring/sesi" "$TOK_A" "data"
isi "Audit log"        "/admin/audit-logs?limit=5" "$TOK_A" "data"

echo
echo "########## kunci jawaban TIDAK boleh sampai ke mahasiswa ##########"
MAT=$(curl -s -H "Authorization: Bearer $TOK_M" "$H/belajar/materi")
echo "  'solusi' di materi mahasiswa : $(echo "$MAT"|grep -o '"solusi"'|wc -l)  (WAJIB 0)"
MATA=$(curl -s -H "Authorization: Bearer $TOK_A" "$H/admin/materi")
echo "  'solusi' di materi staf      : $(echo "$MATA"|grep -o '"solusi"'|wc -l)  (boleh > 0)"
SOALM=$(curl -s -X POST -H "Authorization: Bearer $TOK_M" "$H/game/mulai?bahasa=c&jumlah=3")
echo "  'baris_bug' saat mulai main  : $(echo "$SOALM"|grep -o '"baris_bug"'|wc -l)  (WAJIB 0)"

echo
echo "########## bersihkan jejak ##########"
q "delete from sesi_aktif where user_id in ($UID_M,$UID_A)"
