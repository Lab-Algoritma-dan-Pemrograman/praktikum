package usecase

import (
	"context"
	"html"
	"regexp"
	"strings"
	"time"

	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
	"lab-ap/pkg/ollama"
)

// RumpangUsecase: watcher kode rumpang untuk soal coding keterampilan kelas
// karyawan (TE AD / TE AE). Cron berkala mencari soal coding keterampilan yang
// punya kunci tapi belum punya kode_rumpang, lalu meminta LLM membuat versi
// rumpang dari kuncinya dan menyimpannya.
type RumpangUsecase struct {
	soal   repository.SoalRepository
	course repository.CourseRepository
	llm    *ollama.Client
}

func NewRumpangUsecase(s repository.SoalRepository, c repository.CourseRepository, llm *ollama.Client) *RumpangUsecase {
	return &RumpangUsecase{soal: s, course: c, llm: llm}
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

// kodeBersih mengekstrak kode C murni dari kunci jawaban (yang disimpan sebagai
// HTML editor: teks + blok <pre><code class="language-c">...</code></pre>).
// Mengembalikan blok kode terbesar; "" bila tidak ada blok kode.
func kodeBersih(kunci string) string {
	blocks := []string{}
	rest := kunci
	for {
		i := strings.Index(rest, "<code")
		if i < 0 {
			break
		}
		j := strings.Index(rest[i:], ">")
		if j < 0 {
			break
		}
		start := i + j + 1
		k := strings.Index(rest[start:], "</code>")
		if k < 0 {
			break
		}
		blocks = append(blocks, html.UnescapeString(rest[start:start+k]))
		rest = rest[start+k+len("</code>"):]
	}
	best := ""
	for _, b := range blocks {
		// Blok kode C asli mengandung kurung kurawal / #include / titik-koma.
		if strings.Contains(b, "{") && len(b) > len(best) {
			best = b
		}
	}
	return strings.TrimSpace(best)
}

// instruksiBersih mengubah teks soal HTML menjadi teks instruksi polos.
func instruksiBersih(teks string) string {
	t := tagRe.ReplaceAllString(teks, " ")
	t = html.UnescapeString(t)
	t = strings.Join(strings.Fields(t), " ")
	return t
}

const promptRumpang = `Kamu menerima INSTRUKSI soal pemrograman C dan KUNCI (kode C lengkap yang benar).

Tugas: buat versi RUMPANG dari kode kunci untuk mahasiswa kelas karyawan.
Aturan rumpang (WAJIB dipatuhi):
1. Salin struktur kode kunci apa adanya (urutan baris, fungsi, blok main).
2. Ganti SEBAGIAN token dengan blank: "__" untuk token pendek (nama variabel,
   tipe data, operator, angka, format string pendek) dan "____" untuk token
   panjang (nama fungsi, parameter fungsi, string format panjang, statement).
3. Blank HARUS acak jenisnya: campur tipe data, nama variabel, nama fungsi /
   parameter, fungsi printf/scanf, argumen printf/scanf, operator, dan nilai
   return. Jangan hanya satu jenis (misal jangan semua blank berupa tipe data).
4. Biarkan string prompt yang panjang (misal "Masukkan nama Anda: ") tetap utuh
   sebagai petunjuk — yang di-blank adalah bagian kode di sekitarnya.
5. JANGAN tambah komentar apa pun (tanpa //, tanpa /* */). Mahasiswa hanya
   mengandalkan instruksi soal.
6. Kode harus tetap berbentuk program C yang valid strukturnya (include, fungsi,
   main, return) — hanya isinya berlubang.
7. Keluarkan HANYA kode rumpang, tanpa penjelasan, tanpa markdown fence.

INSTRUKSI:
%s

KUNCI:
%s`

// buatRumpang meminta LLM membuat kode rumpang dari instruksi + kunci.
func (uc *RumpangUsecase) buatRumpang(ctx context.Context, instruksi, kunci string) (string, error) {
	if uc.llm == nil {
		return "", ErrBadRequest
	}
	prompt := strings.Replace(promptRumpang, "%s", instruksi, 1)
	prompt = strings.Replace(prompt, "%s", kunci, 1)
	out, err := uc.llm.RawChat(ctx, prompt)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	// Kupas markdown fence bila LLM membungkus (harusnya tidak, tapi defensif).
	out = strings.TrimPrefix(out, "```c")
	out = strings.TrimPrefix(out, "```")
	out = strings.TrimSuffix(out, "```")
	return strings.TrimSpace(out), nil
}

// Sapu memproses soal coding keterampilan yang punya kunci tapi belum punya
// kode_rumpang. Mengembalikan jumlah soal yang berhasil dibuatkan rumpang.
// Idempoten: soal yang sudah punya kode_rumpang dilewati.
func (uc *RumpangUsecase) Sapu(ctx context.Context) (int, []int, error) {
	courses, err := uc.course.ListByJenis(entity.CourseKeterampilan)
	if err != nil {
		return 0, nil, err
	}
	n := 0
	var gagal []int
	for _, c := range courses {
		pool, err := uc.soal.PoolAll(c.ID)
		if err != nil {
			continue
		}
		for _, s := range pool {
			if s.JenisSoal != entity.SoalCoding {
				continue
			}
			if s.KunciJawaban == nil || strings.TrimSpace(*s.KunciJawaban) == "" {
				continue
			}
			if s.KodeRumpang != nil && strings.TrimSpace(*s.KodeRumpang) != "" {
				continue
			}
			kode := kodeBersih(*s.KunciJawaban)
			if kode == "" {
				continue
			}
			ctx2, cancel := context.WithTimeout(ctx, 3*time.Minute)
			rumpang, err := uc.buatRumpang(ctx2, instruksiBersih(s.TeksSoal), kode)
			cancel()
			if err != nil || rumpang == "" {
				gagal = append(gagal, s.ID)
				continue
			}
			s.KodeRumpang = &rumpang
			if err := uc.soal.Update(&s); err != nil {
				gagal = append(gagal, s.ID)
				continue
			}
			n++
		}
	}
	return n, gagal, nil
}
