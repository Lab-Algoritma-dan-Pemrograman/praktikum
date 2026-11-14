package main

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func inspectQuality(ctx context.Context, tx pgx.Tx, r *report) error {
	columns := map[string]bool{}
	for i := range r.Tables {
		t := &r.Tables[i]
		t.Latest = map[string]*string{}
		for _, c := range t.Columns {
			columns[t.Name+"."+c.Name] = true
			if !strings.HasPrefix(c.Type, "timestamp") {
				continue
			}
			switch c.Name {
			case "updated_at", "check_in_time", "played_at", "completed_at", "waktu_submit", "last_active", "created_at":
			default:
				continue
			}
			var value *string
			if err := tx.QueryRow(ctx, "select max("+pgx.Identifier{c.Name}.Sanitize()+")::text from "+pgx.Identifier{"public", t.Name}.Sanitize()).Scan(&value); err != nil {
				return errors.New("bounded freshness query failed")
			}
			t.Latest[c.Name] = value
		}
	}
	type probe struct {
		name, sql string
		requires  []string
	}
	probes := []probe{
		{"duplicate_nonblank_nim_groups", `select count(*) from (select trim(nim) from public.users where nullif(trim(nim),'') is not null group by trim(nim) having count(*)>1) x`, []string{"users.nim"}},
		{"duplicate_nonblank_email_groups", `select count(*) from (select lower(trim(email)) from public.users where nullif(trim(email),'') is not null group by lower(trim(email)) having count(*)>1) x`, []string{"users.email"}},
		{"missing_nim_users", `select count(*) from public.users where nullif(trim(nim),'') is null`, []string{"users.nim"}},
		{"fractional_question_points", `select count(*) from public.soal where poin<>trunc(poin)`, []string{"soal.poin"}},
		{"fractional_answer_grades", `select count(*) from public.jawaban_mahasiswa where nilai<>trunc(nilai)`, []string{"jawaban_mahasiswa.nilai"}},
		{"fractional_attempt_totals", `select count(*) from public.pengerjaan_course where total_nilai<>trunc(total_nilai)`, []string{"pengerjaan_course.total_nilai"}},
		{"progress_missing_user", `select count(*) from public.student_progress p where not exists(select 1 from public.users u where u.nim=p.nim)`, []string{"student_progress.nim", "users.nim"}},
		{"progress_ambiguous_user", `select count(*) from public.student_progress p where (select count(*) from public.users u where u.nim=p.nim)>1`, []string{"student_progress.nim", "users.nim"}},
		{"progress_missing_lesson", `select count(*) from public.student_progress p where not exists(select 1 from public.lessons l where l.id=p.lesson_id)`, []string{"student_progress.lesson_id", "lessons.id"}},
		{"completed_progress", `select count(*) from public.student_progress where completed`, []string{"student_progress.completed"}},
		{"flowchart_lessons", `select count(*) from public.lessons where exercise_type='flowchart'`, []string{"lessons.exercise_type"}},
		{"unrecognized_legacy_roles", `select count(*) from public.users where role not in ('mahasiswa','asisten','koordinator','peminjam','admin','praktikan','mhs','kordas','korda','coordinator','penyewa','user') or role is null`, []string{"users.role"}},
	}
	r.Quality = map[string]int64{}
	for _, p := range probes {
		ok := true
		for _, required := range p.requires {
			ok = ok && columns[required]
		}
		if !ok {
			continue
		}
		var n int64
		if err := tx.QueryRow(ctx, p.sql).Scan(&n); err != nil {
			return errors.New("quality aggregate failed: " + p.name)
		}
		r.Quality[p.name] = n
	}
	return nil
}
