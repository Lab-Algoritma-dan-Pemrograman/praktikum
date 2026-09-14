#!/bin/bash
# Sapu SEMUA endpoint yang dipanggil frontend elearning.
# AMAN: dikirim TANPA token. Gin menentukan 404/405 di lapisan routing,
# SEBELUM middleware auth dan sebelum handler jalan -- jadi tidak ada
# satu pun data produksi yang tersentuh.
#   401 = route ADA (ditahan auth)   -> benar
#   404 = route HILANG               -> bug
#   405 = metode SALAH               -> bug
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
PORT=8094
export PORT APP_ENV=development
H="http://127.0.0.1:$PORT/api"
T="$LOCALAPPDATA/Temp"

go build -o "$T/labap_s.exe" ./cmd/server || exit 1
"$T/labap_s.exe" > "$T/labap_s.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null' EXIT
for i in $(seq 1 40); do curl -s -o /dev/null "$H/health" 2>/dev/null && break; sleep 0.5; done

GAGAL=0
cek() { # metode path
  local m="$1" p="$2" code
  code=$(curl -s -o /dev/null -w "%{http_code}" -X "$m" "$H$p")
  case "$code" in
    404|405) echo "  BUG  $code  $m $p"; GAGAL=$((GAGAL+1)) ;;
    401|403) echo "  ok   $code  $m $p" ;;
    *)       echo "  ??   $code  $m $p  (route ada, tapi lolos auth -- periksa)" ;;
  esac
}

echo "########## jalur mahasiswa ##########"
cek GET  "/belajar/sesi"
cek GET  "/belajar/level"
cek GET  "/belajar/modul"
cek GET  "/belajar/materi"
cek GET  "/belajar/progres"
cek GET  "/belajar/pencapaian"
cek POST "/belajar/materi/1/selesai"
cek GET  "/game/konfigurasi"
cek GET  "/game/status"
cek POST "/game/mulai"
cek POST "/game/selesai"
cek GET  "/game/peringkat"
cek POST "/game/detak"

echo
echo "########## jalur staf/admin ##########"
cek GET    "/admin/belajar/users"
cek GET    "/admin/belajar/users/1/progres"
cek PUT    "/admin/belajar/users/1/akses-level"
cek POST   "/admin/belajar/users/1/reset"
cek PUT    "/admin/belajar/users/1/xp"
cek PUT    "/admin/belajar/kurikulum"
cek DELETE "/admin/belajar/kurikulum"
cek GET    "/admin/materi"
cek GET    "/admin/materi/1"
cek POST   "/admin/materi"
cek PUT    "/admin/materi/1"
cek DELETE "/admin/materi/1"
cek GET    "/admin/game/soal"
cek POST   "/admin/game/soal"
cek PUT    "/admin/game/soal/1"
cek DELETE "/admin/game/soal/1"
cek PUT    "/admin/game/konfigurasi"
cek GET    "/admin/monitoring/sesi"
cek GET    "/admin/audit-logs"
cek DELETE "/admin/users/1"

echo
echo "########## hasil ##########"
if [ "$GAGAL" -eq 0 ]; then
  echo "  TIDAK ADA route hilang / metode salah."
else
  echo "  $GAGAL endpoint BERMASALAH."
fi
