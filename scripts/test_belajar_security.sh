#!/bin/bash
# Uji keamanan: mahasiswa TIDAK boleh menyentuh kunci jawaban.
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
PORT=8099
export PORT APP_ENV=development

go run ./cmd/server >/tmp/srv2.log 2>&1 &
SRV=$!
trap "kill $SRV 2>/dev/null" EXIT
for i in $(seq 1 60); do curl -s --max-time 2 "http://127.0.0.1:$PORT/api/health" >/dev/null 2>&1 && break; sleep 1; done

# token MAHASISWA (Rakha, id 1326, role mahasiswa)
MHS=$(go run ./cmd/mktoken -uid 1326 -nim 202511011 -role mahasiswa)
# token ASISTEN (Faqod, id 1150)
ASI=$(go run ./cmd/mktoken -uid 1150 -nim 202314020 -role asisten)

echo "=== MAHASISWA -> /admin/materi (HARUS 403) ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/admin/materi" -H "Authorization: Bearer $MHS" -o /dev/null -w "http=%{http_code}\n"

echo "=== MAHASISWA -> /admin/materi/c-level-1-m1-l1 (HARUS 403) ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/admin/materi/c-level-1-m1-l1" -H "Authorization: Bearer $MHS" -o /dev/null -w "http=%{http_code}\n"

echo "=== MAHASISWA -> DELETE /admin/materi/... (HARUS 403, materi tidak boleh terhapus) ==="
curl -s --max-time 20 -X DELETE "http://127.0.0.1:$PORT/api/admin/materi/c-level-1-m1-l1" -H "Authorization: Bearer $MHS" -o /dev/null -w "http=%{http_code}\n"

echo "=== MAHASISWA -> /belajar/materi (boleh, tapi TANPA kunci) ==="
OUT="$LOCALAPPDATA/Temp/mhs_materi.json"
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/belajar/materi?modul_id=c-level-1-m1" -H "Authorization: Bearer $MHS" -o "$OUT" -w "http=%{http_code}\n"
echo "ukuran respons: $(wc -c < "$OUT") byte"
echo "kemunculan kata 'solusi' di byte mentah: $(grep -o 'solusi' "$OUT" | wc -l)  (harus 0)"
echo "kemunculan kata 'solution'            : $(grep -o 'solution' "$OUT" | wc -l)  (harus 0)"

echo "=== ASISTEN -> /admin/materi (HARUS 200, kunci ada) ==="
OUT2="$LOCALAPPDATA/Temp/asi_materi.json"
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/admin/materi?modul_id=c-level-1-m1" -H "Authorization: Bearer $ASI" -o "$OUT2" -w "http=%{http_code}\n"
echo "kemunculan kata 'solusi' di respons asisten: $(grep -o '\"solusi\"' "$OUT2" | wc -l)  (harus > 0)"

echo "=== materi masih utuh 96? ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/admin/materi" -H "Authorization: Bearer $ASI" | python -c "import sys,json;d=json.load(sys.stdin);print('total materi:',len(d.get('data') or []))"
echo "SELESAI"
