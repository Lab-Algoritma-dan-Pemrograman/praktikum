package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Every selected column is explicit. Passwords, bearer tokens, contact/profile
// links, authentication subjects and audit payloads never enter this snapshot.
var snapshotColumns = map[string]string{
	"users":                 "id role nim nama email divisi kelas_id shift is_aktif is_active tersembunyi xp level streak last_active prestige_tier prestige_mult prestige_count class_code major created_at updated_at",
	"levels":                "id title description access_mode locked sort_order",
	"modules":               "id level_id title sort_order",
	"lessons":               "id module_id title explanation code_example initial_code solution hint quiz test_cases validation_rules sort_order xp_reward exercise_type flowchart_distractors",
	"student_progress":      "id nim lesson_id completed completed_at",
	"game_questions":        "id language difficulty title code bug_line explanation",
	"game_history":          "id nim game_type xp_earned played_at",
	"achievements":          "id title description requirement_type requirement_value icon",
	"unlocked_achievements": "id nim achievement_id unlocked_at",
	"kelas":                 "id nama_kelas",
	"jadwal":                "id kelas_id shift hari jam_mulai jam_selesai",
	"sesi_praktikum":        "id judul_sesi urutan is_active created_at updated_at",
	"course":                "id sesi_praktikum_id jenis judul deskripsi durasi_menit kuota_easy kuota_medium kuota_hard",
	"soal":                  "id course_id jenis_soal difficulty kategori_ujian teks_soal poin kunci_jawaban created_at is_arsip kode_rumpang",
	"aktivasi_sesi":         "id sesi_praktikum_id kelas_id shift is_active activated_at gelombang",
	"aktivasi_course":       "id aktivasi_sesi_id course_id is_open urutan opened_at closed_at started_at",
	"soal_terpilih":         "id aktivasi_sesi_id course_id soal_id urutan",
	"pengerjaan_course":     "id mahasiswa_id aktivasi_sesi_id course_id status waktu_mulai waktu_selesai total_nilai keaktifan",
	"jawaban_mahasiswa":     "id mahasiswa_id soal_terpilih_id jawaban_teks is_submitted nilai feedback waktu_submit updated_at",
	"peserta_susulan":       "id mahasiswa_id aktivasi_sesi_id course_id created_at",
	"schedules":             "id title type day_of_week start_time end_time room status major class_code updated_at",
	"group_members":         "id schedule_id user_id student_id group_number kelompok created_at",
	"group_assistants":      "id schedule_id user_id assistant_id created_at",
	"qr_sessions":           "id title is_active created_at created_by",
	"attendance_logs":       "id custom_user_id check_in_time status is_verified session_id verification_status reschedule_schedule_id reschedule_status type recorded_by meeting",
	"inventory_items":       "id name condition quantity location last_checked is_rentable price_per_day updated_at",
	"inventory_rentals":     "id user_id item_id quantity start_date end_date type total_price status created_at updated_at",
}
var userFields = map[string]bool{"mahasiswa_id": true, "custom_user_id": true, "recorded_by": true, "created_by": true, "student_id": true, "assistant_id": true}
var directPII = map[string]bool{"nama": true, "email": true, "nim": true}
var emailPattern = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
var phonePattern = regexp.MustCompile(`(?:\+62|08|628)[0-9 ()-]{7,18}`)

func collectSnapshot(ctx context.Context, tx pgx.Tx, r *report) (map[string][]map[string]any, error) {
	raw := map[string][]map[string]any{}
	totalBytes := 0
	for _, t := range r.Tables {
		requested, ok := snapshotColumns[t.Name]
		if !ok {
			continue
		}
		available := map[string]bool{}
		for _, c := range t.Columns {
			available[c.Name] = true
		}
		cols := []string{}
		for _, name := range strings.Fields(requested) {
			if available[name] {
				cols = append(cols, pgx.Identifier{name}.Sanitize())
			}
		}
		if !available["id"] || len(cols) == 0 {
			return nil, errors.New("snapshot schema lacks required primary key")
		}
		q := "select to_jsonb(selected) from (select " + strings.Join(cols, ",") + " from " + pgx.Identifier{"public", t.Name}.Sanitize() + " order by id) selected"
		rows, e := tx.Query(ctx, q)
		if e != nil {
			return nil, errors.New("snapshot read failed for " + t.Name)
		}
		raw[t.Name] = []map[string]any{}
		for rows.Next() {
			var body []byte
			if e = rows.Scan(&body); e != nil {
				rows.Close()
				return nil, errors.New("snapshot read failed")
			}
			totalBytes += len(body)
			if totalBytes > 64<<20 {
				rows.Close()
				return nil, errors.New("snapshot exceeds 64MiB bound")
			}
			var row map[string]any
			if e = json.Unmarshal(body, &row); e != nil {
				rows.Close()
				return nil, errors.New("snapshot JSON decode failed")
			}
			raw[t.Name] = append(raw[t.Name], row)
		}
		rows.Close()
		if rows.Err() != nil {
			return nil, errors.New("snapshot stream failed")
		}
		if int64(len(raw[t.Name])) != t.Rows {
			return nil, errors.New("snapshot/count disagreement")
		}
	}
	return raw, nil
}

func maskRows(raw map[string][]map[string]any) error {
	if len(raw["users"]) == 0 {
		return errors.New("source identity set empty")
	}
	// The source identity map is held only in memory. Random opaque IDs are fixed
	// in the captured artifact; repeat transforms/imports use that same artifact.
	idMap := map[int64]int64{}
	usedIDs := map[int64]bool{}
	nimMap := map[string]string{}
	replacements := map[string]string{}
	for _, u := range raw["users"] {
		id, ok := u["id"].(float64)
		if !ok || id <= 0 || id != float64(int64(id)) {
			return errors.New("unsupported source user identifier")
		}
		// This is pseudonymization, not a claim of irreversible anonymization.
		masked := int64(0)
		for masked == 0 || usedIDs[masked] {
			n, err := rand.Int(rand.Reader, big.NewInt(900000000))
			if err != nil {
				return errors.New("cannot generate opaque migration identity")
			}
			masked = 1000000000 + n.Int64()
		}
		usedIDs[masked] = true
		idMap[int64(id)] = masked
		alias := fmt.Sprintf("Migration User %d", masked)
		if n, ok := u["nim"].(string); ok && n != "" {
			nimMap[n] = fmt.Sprintf("MIG%d", masked)
			replacements[n] = nimMap[n]
		}
		if n, ok := u["nama"].(string); ok && len(strings.TrimSpace(n)) >= 3 {
			replacements[n] = alias
		}
		if n, ok := u["email"].(string); ok && n != "" {
			replacements[n] = fmt.Sprintf("migration.%d@example.test", masked)
		}
	}
	words := make([]string, 0, len(replacements))
	for s := range replacements {
		words = append(words, s)
	}
	sort.Slice(words, func(i, j int) bool {
		if len(words[i]) == len(words[j]) {
			return words[i] < words[j]
		}
		return len(words[i]) > len(words[j])
	})
	escaped := make([]string, len(words))
	for i, s := range words {
		escaped[i] = regexp.QuoteMeta(s)
	}
	known := regexp.MustCompile("(?i)" + strings.Join(escaped, "|"))
	lower := map[string]string{}
	for k, v := range replacements {
		lower[strings.ToLower(k)] = v
	}
	maskString := func(s string) string {
		s = known.ReplaceAllStringFunc(s, func(v string) string { return lower[strings.ToLower(v)] })
		s = emailPattern.ReplaceAllString(s, "masked@example.test")
		return phonePattern.ReplaceAllString(s, "[masked-contact]")
	}
	for name, rows := range raw {
		for _, row := range rows {
			for k, v := range row {
				if name == "users" && k == "id" {
					row[k] = idMap[int64(v.(float64))]
					continue
				}
				isUser := userFields[k] || k == "user_id" && (name == "group_members" || name == "group_assistants" || name == "inventory_rentals")
				if isUser && v != nil {
					if n, ok := v.(float64); ok {
						mapped, exists := idMap[int64(n)]
						if !exists && n != 0 {
							return errors.New("source user reference is unresolved")
						}
						row[k] = mapped
					} else {
						return errors.New("unsupported source user reference type")
					}
					continue
				}
				if k == "nim" && v != nil {
					n, ok := v.(string)
					if !ok {
						return errors.New("invalid NIM type")
					}
					mapped, exists := nimMap[n]
					if !exists {
						return errors.New("NIM reference is unresolved")
					}
					row[k] = mapped
					continue
				}
				if name == "users" && (k == "nama" || k == "email") {
					sourceID := int64(0)
					switch n := row["id"].(type) {
					case float64:
						sourceID = idMap[int64(n)]
					case int64:
						sourceID = n
					}
					if k == "nama" {
						row[k] = "Migration User " + strconv.FormatInt(sourceID, 10)
					} else {
						row[k] = fmt.Sprintf("migration.%d@example.test", sourceID)
					}
					continue
				}
				// Learner-authored free text can contain personal or health information.
				// Preserve its character count, but never persist its original content.
				if name == "jawaban_mahasiswa" && (k == "jawaban_teks" || k == "feedback") {
					if s, ok := v.(string); ok && s != "" {
						row[k] = fmt.Sprintf("[Masked legacy %s: %d characters]", k, len([]rune(s)))
					}
					continue
				}
				if s, ok := v.(string); ok && !strings.HasSuffix(k, "_id") && k != "id" {
					row[k] = maskString(s)
				}
			}
		}
	}
	// The intermediate still contains educational content, pseudonymous relations
	// and historical grades. Treat it as private; only normalized reviewed fields
	// may be uploaded to staging. No raw working file has been created.
	return nil
}
