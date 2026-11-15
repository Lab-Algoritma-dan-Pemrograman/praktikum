package usecase

import (
	"context"
	"errors"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

// RumpangGenerator is the bounded LLM boundary for fill-in-the-blank generation.
// The service owns permission checks, CAS versioning and audit; the generator
// only converts (instruction, reference code) into a blanked template.
type RumpangGenerator interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Port of legacy rumpang rules (project_ap rumpang_usecase.go): copy the key's
// structure, blank a mixed subset of tokens with __ / ____, keep long string
// prompts intact, no comments, output code only.
const rumpangSystemPrompt = `Kamu menerima INSTRUKSI soal pemrograman C dan KUNCI (kode C lengkap yang benar).

Tugas: buat versi RUMPANG dari kode kunci untuk mahasiswa kelas karyawan.
Aturan rumpang (WAJIB dipatuhi):
1. Salin struktur kode kunci apa adanya (urutan baris, fungsi, blok main).
2. Ganti SEBAGIAN token dengan blank: "__" untuk token pendek (nama variabel, tipe data, operator, angka, format string pendek) dan "____" untuk token panjang (nama fungsi, parameter fungsi, string format panjang, statement).
3. Blank HARUS acak jenisnya: campur tipe data, nama variabel, nama fungsi / parameter, fungsi printf/scanf, argumen printf/scanf, operator, dan nilai return. Jangan hanya satu jenis.
4. Biarkan string prompt yang panjang (misal "Masukkan nama Anda: ") tetap utuh sebagai petunjuk — yang di-blank adalah bagian kode di sekitarnya.
5. JANGAN tambah komentar apa pun (tanpa //, tanpa /* */). Mahasiswa hanya mengandalkan instruksi soal.
6. Kode harus tetap berbentuk program C yang valid strukturnya (include, fungsi, main, return) — hanya isinya berlubang.
7. Keluarkan HANYA kode rumpang, tanpa penjelasan, tanpa markdown fence.

INSTRUKSI:
%s

KUNCI:
%s`

var rumpangTagRe = regexp.MustCompile(`<[^>]*>`)

// rumpangCode extracts the largest C code block from an HTML-stored key.
func rumpangCode(kunci string) string {
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
		if strings.Contains(b, "{") && len(b) > len(best) {
			best = b
		}
	}
	return strings.TrimSpace(best)
}

func rumpangInstruction(teks string) string {
	t := rumpangTagRe.ReplaceAllString(teks, " ")
	t = html.UnescapeString(t)
	return strings.Join(strings.Fields(t), " ")
}

// GenerateRumpang fills an empty StarterCode for one coding question from its
// reference answer. Idempotent: a question that already has starter code is
// rejected instead of silently overwritten.
func (s *PracticumService) GenerateRumpang(ctx context.Context, p *entity.Principal, c PracticumCommand, id string) (entity.PracticumQuestion, error) {
	gen := s.options.Rumpang
	if gen == nil {
		return entity.PracticumQuestion{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.question.rumpang", id, map[string]string{"id": id}, func(tx repository.PracticumTx, replay string) (out entity.PracticumQuestion, target string, scope entity.Scope, err error) {
		q, e := practicumRead[entity.PracticumQuestion](tx, repository.PracticumQuestions, id, true)
		if e != nil {
			err = e
			return
		}
		course, e := practicumRead[entity.PracticumCourse](tx, repository.PracticumCourses, q.CourseID, false)
		if e != nil {
			err = e
			return
		}
		session, e := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, course.SessionID, false)
		if e != nil {
			err = e
			return
		}
		scope = session.Scope
		if err = practicumRequire(p, practicumQuestions, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = practicumRead[entity.PracticumQuestion](tx, repository.PracticumQuestions, replay, false)
			return out, replay, scope, err
		}
		if q.Type != "coding" || q.Language != "c" || strings.TrimSpace(q.ReferenceAnswer) == "" {
			err = ErrValidation
			return
		}
		if strings.TrimSpace(q.StarterCode) != "" {
			err = ErrValidation
			return
		}
		kode := rumpangCode(q.ReferenceAnswer)
		if kode == "" {
			err = ErrValidation
			return
		}
		prompt := strings.Replace(rumpangSystemPrompt, "%s", rumpangInstruction(q.Prompt), 1)
		prompt = strings.Replace(prompt, "%s", kode, 1)
		genCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		template, genErr := gen.Complete(genCtx, "Kamu hanya mengeluarkan kode C.", prompt)
		cancel()
		if genErr != nil || strings.TrimSpace(template) == "" {
			err = errors.New("RUMPANG_GENERATION_FAILED")
			return
		}
		template = strings.TrimSpace(template)
		template = strings.TrimPrefix(template, "```c")
		template = strings.TrimPrefix(template, "```")
		template = strings.TrimSuffix(template, "```")
		template = strings.TrimSpace(template)
		if template == "" || len(template) > 64<<10 {
			err = errors.New("RUMPANG_GENERATION_FAILED")
			return
		}
		if q.Version < 1 {
			q.Version = 1
		}
		q.StarterCode = template
		q.Version++
		out = q
		target = id
		err = practicumPut(tx, repository.PracticumQuestions, id, q.CourseID, 0, scope, "", "", nil, q)
		return
	})
}

var _ = authorization.ErrForbidden // keep import parity with sibling files
