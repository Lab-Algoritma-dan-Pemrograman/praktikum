// regrade: AI grading massal untuk SELURUH jawaban (arsip + aktif) langsung ke AI,
// tanpa lewat browser. Worker pool, resumable (hanya grade yang nilai IS NULL),
// lalu recalc total_nilai semua pengerjaan (TERMASUK arsip shift=0).
//
// Pakai:
//
//	go run ./cmd/regrade -backup            # snapshot nilai sekarang (selalu disarankan)
//	go run ./cmd/regrade -reset -grade -recalc           # dari awal: kosongkan -> grade -> recalc
//	go run ./cmd/regrade -grade -recalc                  # lanjut (resume) yang belum dinilai
//	go run ./cmd/regrade -grade -limit 20                # uji coba 20 dulu
//
// Flag: -arsip=false untuk hanya jawaban aktif (non-arsip).
package main

import (
	"context"
	"flag"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"lab-ap/config"
	"lab-ap/database"
	"lab-ap/pkg/ollama"

	"gorm.io/gorm"
)

type row struct {
	ID          int
	JawabanTeks string
	TeksSoal    string
	Kunci       string
	Poin        float64
	JenisSoal   string
	Difficulty  string
	CourseJenis string
}

func main() {
	doBackup := flag.Bool("backup", false, "snapshot nilai+total ke tabel _bak_* lalu lanjut")
	doSetPoin := flag.Bool("setpoin", false, "isi poin soal arsip sesuai rubrik (jenis course + difficulty)")
	doReset := flag.Bool("reset", false, "kosongkan semua nilai+feedback dulu (grading dari awal)")
	doGrade := flag.Bool("grade", false, "jalankan AI grading")
	doRecalc := flag.Bool("recalc", false, "recalc total_nilai semua pengerjaan (termasuk arsip)")
	includeArsip := flag.Bool("arsip", true, "ikut grade jawaban arsip (shift=0)")
	sesi := flag.Int("sesi", 0, "batasi ke 1 sesi_praktikum_id (0=semua)")
	concurrency := flag.Int("concurrency", 2, "jumlah worker paralel (ollama.com rate-limit; >3 banyak gagal)")
	limit := flag.Int("limit", 0, "batasi jumlah jawaban yang digrade (0=semua)")
	flag.Parse()

	cfg := config.Load()
	db, err := database.Connect(cfg.DSN(), false)
	if err != nil {
		log.Fatalf("Koneksi DB gagal: %v", err)
	}
	client := ollama.NewClient(cfg)

	if *doBackup {
		backup(db)
	}
	if *doSetPoin {
		setPoin(db, *sesi)
	}
	if *doReset {
		reset(db, *includeArsip, *sesi)
	}
	if *doGrade {
		grade(db, client, *includeArsip, *sesi, *concurrency, *limit)
	}
	if *doRecalc {
		recalc(db, *includeArsip, *sesi)
	}
	if !*doBackup && !*doSetPoin && !*doReset && !*doGrade && !*doRecalc {
		log.Println("Tidak ada aksi. Lihat -h untuk flag (mis: -backup -setpoin -reset -grade -recalc).")
	}
}

// sesiClause membatasi ke 1 sesi via course_id (kolom bisa st.course_id / pc.course_id / so.course_id).
func sesiClause(col string, sesi int) string {
	if sesi <= 0 {
		return ""
	}
	return " AND " + col + " IN (SELECT id FROM course WHERE sesi_praktikum_id=" + strconv.Itoa(sesi) + ")"
}

// setPoin mengisi poin soal arsip sesuai rubrik AI (lihat rubrikPoin di FE sesi).
func setPoin(db *gorm.DB, sesi int) {
	q := `UPDATE soal so SET poin = CASE
			WHEN c.jenis='keterampilan' THEN 85
			WHEN c.jenis='ujian_praktik' THEN 45
			WHEN c.jenis='pretest'  AND so.difficulty='medium' THEN 15
			WHEN c.jenis='pretest'  AND so.difficulty='hard'   THEN 25
			WHEN c.jenis='pretest'  THEN 20
			WHEN c.jenis='posttest' AND so.difficulty='medium' THEN 35
			WHEN c.jenis='posttest' AND so.difficulty='hard'   THEN 45
			WHEN c.jenis='posttest' THEN 20
			ELSE 20 END
		FROM course c WHERE c.id=so.course_id AND so.is_arsip=true` + sesiClause("so.course_id", sesi)
	res := db.Exec(q)
	log.Printf("✓ setpoin: %d soal arsip diisi poin rubrik", res.RowsAffected)
}

func arsipFilter(includeArsip bool) string {
	if includeArsip {
		return ""
	}
	return " AND st.aktivasi_sesi_id IN (SELECT id FROM aktivasi_sesi WHERE shift <> 0)"
}

func backup(db *gorm.DB) {
	suffix := time.Now().Format("20060102_150405")
	db.Exec("CREATE TABLE _bak_jawaban_" + suffix + " AS SELECT id, nilai, feedback FROM jawaban_mahasiswa")
	db.Exec("CREATE TABLE _bak_pengerjaan_" + suffix + " AS SELECT id, total_nilai FROM pengerjaan_course")
	log.Printf("✓ backup -> _bak_jawaban_%s, _bak_pengerjaan_%s", suffix, suffix)
}

func reset(db *gorm.DB, includeArsip bool, sesi int) {
	q := `UPDATE jawaban_mahasiswa SET nilai=NULL, feedback=NULL
		WHERE id IN (SELECT j.id FROM jawaban_mahasiswa j JOIN soal_terpilih st ON st.id=j.soal_terpilih_id
		WHERE 1=1` + arsipFilter(includeArsip) + sesiClause("st.course_id", sesi) + `)`
	res := db.Exec(q)
	log.Printf("✓ reset nilai: %d jawaban dikosongkan", res.RowsAffected)
}

func grade(db *gorm.DB, client *ollama.Client, includeArsip bool, sesi, concurrency, limit int) {
	q := `SELECT j.id, j.jawaban_teks,
			so.teks_soal, COALESCE(so.kunci_jawaban,'') kunci, so.poin, so.jenis_soal,
			COALESCE(so.difficulty,'') difficulty, c.jenis course_jenis
		FROM jawaban_mahasiswa j
		JOIN soal_terpilih st ON st.id=j.soal_terpilih_id
		JOIN soal so ON so.id=st.soal_id
		JOIN course c ON c.id=st.course_id
		WHERE j.is_submitted AND COALESCE(j.jawaban_teks,'')<>'' AND j.nilai IS NULL` + arsipFilter(includeArsip) + sesiClause("st.course_id", sesi)
	if limit > 0 {
		q += " LIMIT " + strconv.Itoa(limit)
	}
	var rows []row
	if err := db.Raw(q).Scan(&rows).Error; err != nil {
		log.Fatalf("query target gagal: %v", err)
	}
	total := len(rows)
	log.Printf("Mulai grade %d jawaban (concurrency=%d)…", total, concurrency)
	if total == 0 {
		return
	}

	var done, failed int64
	jobs := make(chan row, concurrency*2)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				var res *ollama.AIResult
				var err error
				for attempt := 1; attempt <= 3; attempt++ { // retry: ollama.com kadang rate-limit/timeout
					res, err = client.GradeAnswer(context.Background(), ollama.GradeParams{
						Soal: r.TeksSoal, Kunci: r.Kunci, Jawaban: r.JawabanTeks,
						Poin: r.Poin, JenisSoal: r.JenisSoal, Difficulty: r.Difficulty, JenisCourse: r.CourseJenis,
					})
					if err == nil {
						break
					}
					time.Sleep(time.Duration(attempt) * time.Second)
				}
				if err != nil {
					if atomic.AddInt64(&failed, 1) <= 5 {
						log.Printf("  GAGAL id=%d: %v", r.ID, err)
					}
				} else {
					fb := "[AI] " + res.Feedback
					db.Exec("UPDATE jawaban_mahasiswa SET nilai=?, feedback=? WHERE id=?", res.Nilai, fb, r.ID)
				}
				n := atomic.AddInt64(&done, 1)
				if n%50 == 0 || int(n) == total {
					log.Printf("  %d/%d (gagal: %d)", n, total, atomic.LoadInt64(&failed))
				}
			}
		}()
	}
	for _, r := range rows {
		jobs <- r
	}
	close(jobs)
	wg.Wait()
	log.Printf("✓ grade selesai: %d sukses, %d gagal (jalankan lagi -grade untuk ulang yang gagal)", done-failed, failed)
}

// recalc set total_nilai = SUM(nilai) untuk SEMUA pengerjaan terdampak, termasuk arsip.
func recalc(db *gorm.DB, includeArsip bool, sesi int) {
	q := `UPDATE pengerjaan_course pc SET total_nilai = sub.s
		FROM (
			SELECT j.mahasiswa_id, st.aktivasi_sesi_id, st.course_id, COALESCE(SUM(j.nilai),0) s
			FROM jawaban_mahasiswa j JOIN soal_terpilih st ON st.id=j.soal_terpilih_id
			GROUP BY j.mahasiswa_id, st.aktivasi_sesi_id, st.course_id
		) sub
		WHERE pc.mahasiswa_id=sub.mahasiswa_id AND pc.aktivasi_sesi_id=sub.aktivasi_sesi_id
		  AND pc.course_id=sub.course_id`
	if !includeArsip {
		q += " AND pc.aktivasi_sesi_id IN (SELECT id FROM aktivasi_sesi WHERE shift <> 0)"
	}
	q += sesiClause("pc.course_id", sesi)
	res := db.Exec(q)
	log.Printf("✓ recalc total_nilai: %d pengerjaan diperbarui", res.RowsAffected)
}
