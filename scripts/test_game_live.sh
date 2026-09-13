#!/bin/bash
# Uji live endpoint game/pencapaian/peringkat/monitoring lawan DB prod.
cd "E:/rekapProject/project_ap/praktikum/backend" || exit 1
PORT=8099
export PORT APP_ENV=development
H="http://127.0.0.1:$PORT/api"

go run ./cmd/server >/tmp/srv3.log 2>&1 &
SRV=$!
trap "kill $SRV 2>/dev/null" EXIT
for i in $(seq 1 60); do curl -s --max-time 2 "$H/health" >/dev/null 2>&1 && break; sleep 1; done
curl -s --max-time 5 "$H/health" >/dev/null || { echo "SERVER MATI"; tail -20 /tmp/srv3.log; exit 1; }
echo "server hidup"

MHS=$(go run ./cmd/mktoken -uid 1326 -nim 202511011 -role mahasiswa)
ASI=$(go run ./cmd/mktoken -uid 1150 -nim 202314020 -role asisten)
AM="Authorization: Bearer $MHS"; AA="Authorization: Bearer $ASI"
T="$LOCALAPPDATA/Temp"

echo
echo "=== GET /game/konfigurasi ==="
curl -s --max-time 20 "$H/game/konfigurasi" -H "$AM" -o "$T/gk.json" -w "http=%{http_code}\n"
python -c "import json;d=json.load(open(r'$T/gk.json'));print(' ',d.get('data'))"

echo "=== GET /game/status ==="
curl -s --max-time 20 "$H/game/status" -H "$AM" -o "$T/gs.json" -w "http=%{http_code}\n"
python -c "import json;d=json.load(open(r'$T/gs.json'));print(' ',d.get('data'))"

echo "=== POST /game/mulai?bahasa=c&jumlah=3  (KUNCI JAWABAN HARUS TIDAK ADA) ==="
curl -s --max-time 20 -X POST "$H/game/mulai?bahasa=c&jumlah=3" -H "$AM" -o "$T/gm.json" -w "http=%{http_code}\n"
python -c "
import json;d=json.load(open(r'$T/gm.json'));x=d.get('data') or {};s=x.get('soal') or []
print('  jumlah soal:',len(s))
if s:
    print('  kolom soal:',sorted(s[0].keys()))
    print('  BOCOR baris_bug?', any('baris_bug' in q for q in s))
    print('  BOCOR penjelasan?', any('penjelasan' in q for q in s))
    print('  id soal:',[q['id'] for q in s])
"
echo "  cek mentah 'baris_bug': $(grep -o 'baris_bug' "$T/gm.json" | wc -l) (harus 0)"
echo "  cek mentah 'penjelasan': $(grep -o 'penjelasan' "$T/gm.json" | wc -l) (harus 0)"

echo
echo "=== ANTI-CURANG: kirim jawaban SALAH semua -> XP harus 0 ==="
IDS=$(python -c "import json;d=json.load(open(r'$T/gm.json'));print(' '.join(str(q['id']) for q in (d.get('data') or {}).get('soal',[])))")
BODY=$(python -c "
import json
ids='$IDS'.split()
print(json.dumps({'jawaban':[{'soal_id':int(i),'baris_dipilih':-999} for i in ids]}))
")
curl -s --max-time 20 -X POST "$H/game/selesai" -H "$AM" -H "Content-Type: application/json" -d "$BODY" -o "$T/sel1.json" -w "http=%{http_code}\n"
python -c "import json;d=json.load(open(r'$T/sel1.json'));x=d.get('data') or {};print('  benar:',x.get('benar'),'/',x.get('total'),'| xp_didapat:',x.get('xp_didapat'),'(harus 0)')"

echo
echo "=== ANTI-CURANG: client TIDAK BISA kirim skor/xp sendiri ==="
curl -s --max-time 20 -X POST "$H/game/selesai" -H "$AM" -H "Content-Type: application/json" \
  -d "{\"jawaban\":[{\"soal_id\":1,\"baris_dipilih\":1}],\"xp_didapat\":999999,\"benar\":9999}" -o "$T/sel2.json" -w "http=%{http_code}\n"
python -c "import json;d=json.load(open(r'$T/sel2.json'));x=d.get('data') or {};print('  xp_didapat yang DIBERI SERVER:',x.get('xp_didapat'),'(bukan 999999)')"

echo
echo "=== GET /game/peringkat ==="
curl -s --max-time 20 "$H/game/peringkat?limit=5" -H "$AM" -o "$T/pr.json" -w "http=%{http_code}\n"
python -c "
import json;d=json.load(open(r'$T/pr.json'));x=d.get('data') or {}
print('  posisi_saya:',x.get('posisi_saya'))
for b in (x.get('peringkat') or [])[:5]: print('   ',b['nim'],b['nama'][:24],'xp',b['xp'])
"

echo
echo "=== GET /belajar/pencapaian (evaluasi di server) ==="
curl -s --max-time 20 "$H/belajar/pencapaian" -H "$AM" -o "$T/ach.json" -w "http=%{http_code}\n"
python -c "
import json;d=json.load(open(r'$T/ach.json'));x=d.get('data') or {}
print('  total pencapaian:',len(x.get('semua') or []))
print('  sudah terbuka  :',len(x.get('terbuka_id') or []))
print('  baru terbuka   :',[a['id'] for a in (x.get('baru_saja') or [])])
"

echo
echo "=== POST /game/detak lalu admin lihat sesi aktif ==="
curl -s --max-time 20 -X POST "$H/game/detak" -H "$AM" -H "Content-Type: application/json" -d '{"aktivitas":"lesson"}' -o /dev/null -w "detak http=%{http_code}\n"
curl -s --max-time 20 "$H/admin/monitoring/sesi" -H "$AA" -o "$T/ses.json" -w "sesi http=%{http_code}\n"
python -c "
import json;d=json.load(open(r'$T/ses.json'));r=d.get('data') or []
print('  sesi aktif:',len(r))
for s in r[:3]: print('   ',s['nim'],s['nama'][:24],'|',s['aktivitas'],'|',s['role'])
"

echo
echo "=== MAHASISWA -> endpoint admin game (HARUS 403) ==="
curl -s --max-time 20 "$H/admin/game/soal" -H "$AM" -o /dev/null -w "  admin/game/soal   http=%{http_code}\n"
curl -s --max-time 20 "$H/admin/monitoring/sesi" -H "$AM" -o /dev/null -w "  monitoring/sesi   http=%{http_code}\n"
curl -s --max-time 20 -X DELETE "$H/admin/game/soal/1" -H "$AM" -o /dev/null -w "  DELETE soal       http=%{http_code}\n"

echo
echo "=== ASISTEN -> admin/game/soal (200, kunci ada) ==="
curl -s --max-time 20 "$H/admin/game/soal?bahasa=c" -H "$AA" -o "$T/asoal.json" -w "http=%{http_code}\n"
python -c "
import json;d=json.load(open(r'$T/asoal.json'));r=d.get('data') or []
print('  jumlah soal c:',len(r))
if r: print('  ada baris_bug?', 'baris_bug' in r[0], '| ada penjelasan?', 'penjelasan' in r[0])
"

echo
echo "=== bahasa ngawur -> 400 ==="
curl -s --max-time 20 -X POST "$H/game/mulai?bahasa=kobol" -H "$AM" -o /dev/null -w "  http=%{http_code}\n"
echo "=== tanpa token -> 401 ==="
curl -s --max-time 20 "$H/game/status" -o /dev/null -w "  http=%{http_code}\n"
echo "SELESAI"
