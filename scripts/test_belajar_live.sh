#!/bin/bash
# Uji endpoint /belajar live lawan DB PROD. Server lokal.
# Read-only kecuali satu POST /selesai (dipilih materi yang sudah selesai -> XP tak berubah).
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
PORT=8099
export PORT APP_ENV=development

go run ./cmd/server >/tmp/srv.log 2>&1 &
SRV=$!
trap "kill $SRV 2>/dev/null" EXIT

for i in $(seq 1 60); do
  curl -s --max-time 2 "http://127.0.0.1:$PORT/api/health" >/dev/null 2>&1 && break
  sleep 1
done
echo "=== health ==="
curl -s --max-time 5 "http://127.0.0.1:$PORT/api/health" || { echo "SERVER MATI"; tail -20 /tmp/srv.log; exit 1; }
echo

# token uji: Ahmad Faqod Kurnia (id 1150, asisten, punya progres + xp 60)
TOKEN=$(go run ./cmd/mktoken -uid 1150 -nim 202314020 -role asisten 2>/tmp/tok.err)
[ -z "$TOKEN" ] && { echo "TOKEN GAGAL"; cat /tmp/tok.err; exit 1; }
echo "token ok (${#TOKEN} char)"
A="Authorization: Bearer $TOKEN"

echo "=== GET /belajar/level ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/belajar/level" -H "$A" -o /tmp/lv.json -w "http=%{http_code}\n"
python -c "import json;d=json.load(open('/tmp/lv.json'));r=d.get('data') or [];print('jumlah:',len(r));[print(' ',x['id'],'|',x['judul'][:45]) for x in r[:3]]"

echo "=== GET /belajar/modul?level_id=c-level-1 ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/belajar/modul?level_id=c-level-1" -H "$A" -o /tmp/md.json -w "http=%{http_code}\n"
python -c "import json;d=json.load(open('/tmp/md.json'));r=d.get('data') or [];print('jumlah:',len(r));[print(' ',x['id'],'|',x['judul'][:45]) for x in r[:3]]"

echo "=== GET /belajar/materi?modul_id=c-level-1-m1  (KUNCI JAWABAN HARUS TIDAK ADA) ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/belajar/materi?modul_id=c-level-1-m1" -H "$A" -o /tmp/mt.json -w "http=%{http_code}\n"
python -c "
import json;d=json.load(open('/tmp/mt.json'));r=d.get('data') or []
print('jumlah:',len(r))
if r:
    print('kolom:',sorted(r[0].keys()))
    print('BOCOR kunci jawaban?', any(('solusi' in x) or ('solution' in x) for x in r))
    print('contoh:',r[0]['id'],'|',r[0]['judul'][:40],'| xp',r[0].get('xp_hadiah'))
"
echo "cek mentah kata 'solusi' di respons mahasiswa: $(grep -o 'solusi' /tmp/mt.json | wc -l) kemunculan (harus 0)"

echo "=== GET /belajar/progres ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/belajar/progres" -H "$A" -o /tmp/pr.json -w "http=%{http_code}\n"
python -c "import json;d=json.load(open('/tmp/pr.json'));x=d.get('data') or {};print('progres:',len(x.get('progres') or []));print('profil:',x.get('profil'))"

echo "=== GET /admin/materi/c-level-1-m1-l1  (kunci jawaban HARUS ada) ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/admin/materi/c-level-1-m1-l1" -H "$A" -o /tmp/am.json -w "http=%{http_code}\n"
python -c "import json;d=json.load(open('/tmp/am.json'));x=d.get('data') or {};print('ada solusi?',bool(x.get('solusi')),'| panjang:',len(x.get('solusi') or ''))"

echo "=== POST /belajar/materi/c-level-1-m1-l1/selesai  (sudah selesai -> XP TIDAK boleh dobel) ==="
echo "xp sebelum:"; python -c "import json;d=json.load(open('/tmp/pr.json'));print(' ',(d.get('data') or {}).get('profil',{}).get('xp'))"
curl -s --max-time 20 -X POST "http://127.0.0.1:$PORT/api/belajar/materi/c-level-1-m1-l1/selesai" -H "$A" -o /tmp/sel.json -w "http=%{http_code}\n"
python -c "import json;d=json.load(open('/tmp/sel.json'));x=d.get('data') or {};print('xp sesudah:',x.get('xp'),'| level:',x.get('level_angka'))"

echo "=== 404 untuk materi tak ada ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/belajar/materi/tidak-ada-xyz" -H "$A" -o /tmp/nf.json -w "http=%{http_code}\n"

echo "=== tanpa token harus 401 ==="
curl -s --max-time 20 "http://127.0.0.1:$PORT/api/belajar/level" -o /dev/null -w "http=%{http_code}\n"

echo "SELESAI"
