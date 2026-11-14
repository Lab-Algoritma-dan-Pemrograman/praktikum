package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

// SeedLearningFixtures is opt-in local/test data, never a migration or production seed.
func SeedLearningFixtures(ctx context.Context, repo LearningRepository) error {
	return repo.Transaction(ctx, func(tx LearningTx) error {
		if _, err := tx.Get(LearningLevels, "level-1"); err == nil {
			return nil
		} else if !errors.Is(err, ErrLearningNotFound) {
			return err
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		scope := entity.Scope{Domain: "elearning", Institution: "lab", Division: "pendidikan"}
		put := func(kind LearningKind, id, parent, status, unique string, value any) error {
			raw, err := json.Marshal(value)
			if err != nil {
				return err
			}
			return tx.Put(kind, LearningRecord{ID: id, ParentID: parent, Scope: scope, Status: status, UniqueKey: unique, Data: raw})
		}
		levels := []entity.LearningLevel{
			{ID: "level-1", Title: "Level 1 — Dasar Algoritma dan C", Description: "Struktur dasar program C, input/output, dan tipe data.", SortOrder: 1, AccessMode: "unlocked", RequiredXP: 0, Scope: scope, Version: 1, CreatedAt: now, UpdatedAt: now},
			{ID: "level-2", Title: "Level 2 — Percabangan dan Python", Description: "Logika kondisional dan pengenalan Python.", SortOrder: 2, AccessMode: "auto", RequiredXP: 60, Scope: scope, Version: 1, CreatedAt: now, UpdatedAt: now},
			{ID: "level-3", Title: "Level 3 — Struktur Lanjutan (Terkunci)", Description: "Materi lanjutan yang dikunci instruktur.", SortOrder: 3, AccessMode: "locked", RequiredXP: 180, Scope: scope, Version: 1, CreatedAt: now, UpdatedAt: now},
		}
		for _, level := range levels {
			if err = put(LearningLevels, level.ID, "", "", "", level); err != nil {
				return err
			}
		}
		modules := []entity.LearningModule{
			{ID: "module-1", LevelID: "level-1", Title: "Modul 1 — Struktur Dasar C", Description: "Memahami fungsi main, printf, dan penjumlahan.", SortOrder: 1, Scope: scope, Version: 1},
			{ID: "module-2", LevelID: "level-2", Title: "Modul 2 — Percabangan Python", Description: "Evaluasi kondisi if-else pada Python.", SortOrder: 1, Scope: scope, Version: 1},
			{ID: "module-3", LevelID: "level-3", Title: "Modul 3 — Pointer Lanjutan", Description: "Materi terkunci untuk pengujian akses.", SortOrder: 1, Scope: scope, Version: 1},
		}
		for _, mod := range modules {
			if err = put(LearningModules, mod.ID, mod.LevelID, "", "", mod); err != nil {
				return err
			}
		}
		optZero, optOne := 0, 1
		lessons := []entity.LearningLesson{
			{
				ID:          "lesson-1",
				ModuleID:    "module-1",
				LevelID:     "level-1",
				Title:       "Program Pertama dan Output C",
				SortOrder:   1,
				Language:    "c",
				Explanation: "Setiap program C dimulai dari fungsi `main` dan menggunakan `printf` untuk menampilkan keluaran standar.",
				CodeExample: "#include <stdio.h>\nint main(void) {\n  printf(\"Halo Algohub\\n\");\n  return 0;\n}",
				StarterCode: "#include <stdio.h>\nint main(void) {\n  return 0;\n}\n",
				Solution:    "PRIVATE_SOLUTION_LESSON_1_PRINTF",
				Hint:        "Panggil fungsi printf untuk menampilkan teks lalu kembalikan 0 dari main.",
				XPReward:    60,
				Quiz: &entity.LearningQuiz{
					Question:      "Fungsi utama yang pertama kali dieksekusi dalam program C adalah?",
					Options:       []string{"main", "start", "init", "printf"},
					CorrectOption: &optZero,
				},
				TestCases: []entity.LearningTestCase{
					{Description: "Menjalankan kode C dasar", Input: "1 2", ExpectedOutput: "SIMULATED fixture: success scenario; source was not executed."},
				},
				Scope:   scope,
				Version: 1,
			},
			{
				ID:          "lesson-2",
				ModuleID:    "module-2",
				LevelID:     "level-2",
				Title:       "Percabangan If-Else di Python",
				SortOrder:   1,
				Language:    "python",
				Explanation: "Gunakan pernyataan `if` dan `else` dengan indentasi konsisten untuk mengevaluasi kondisi logika.",
				CodeExample: "nilai = 80\nif nilai >= 75:\n    print('Lulus')\nelse:\n    print('Belum lulus')",
				StarterCode: "nilai = 80\n# Tulis percabangan di bawah ini\n",
				Solution:    "PRIVATE_SOLUTION_LESSON_2_PYTHON",
				Hint:        "Periksa kondisi menggunakan operator perbandingan >= dan titik dua (:).",
				XPReward:    80,
				Quiz: &entity.LearningQuiz{
					Question:      "Kata kunci untuk cabang alternatif setelah if di Python adalah?",
					Options:       []string{"otherwise", "else", "then", "switch"},
					CorrectOption: &optOne,
				},
				TestCases: []entity.LearningTestCase{
					{Description: "Menjalankan skrip Python", Input: "80", ExpectedOutput: "SIMULATED fixture: success scenario; source was not executed."},
				},
				Scope:   scope,
				Version: 1,
			},
			{
				ID:          "lesson-3",
				ModuleID:    "module-3",
				LevelID:     "level-3",
				Title:       "Alamat Memori dan Pointer",
				SortOrder:   1,
				Language:    "c",
				Explanation: "Materi pada level terkunci.",
				CodeExample: "int x = 10;\nint *p = &x;",
				StarterCode: "#include <stdio.h>\nint main(void) { return 0; }\n",
				Solution:    "PRIVATE_SOLUTION_LESSON_3_LOCKED",
				Hint:        "Gunakan operator & untuk mengambil alamat.",
				XPReward:    100,
				TestCases:   []entity.LearningTestCase{},
				Scope:       scope,
				Version:     1,
			},
		}
		for _, lesson := range lessons {
			if err = put(LearningLessons, lesson.ID, lesson.ModuleID, "", "", lesson); err != nil {
				return err
			}
		}
		lineFour, lineThree := 4, 3
		bugs := []entity.LearningBugHuntQuestion{
			{
				ID:          "bughunt-c-1",
				Language:    "c",
				Difficulty:  "easy",
				Title:       "Perbandingan vs Penugasan pada If",
				BuggyCode:   "#include <stdio.h>\nint main(void) {\n  int x = 5;\n  if (x = 10) {\n    printf(\"Sepuluh\\n\");\n  }\n  return 0;\n}",
				BugLine:     &lineFour,
				Explanation: "PRIVATE_BUGHUNT_EXPLANATION_C: Menggunakan operator assignment (=) alih-alih perbandingan (==) pada kondisi if.",
				XPReward:    50,
				Active:      true,
				Scope:       scope,
				Version:     1,
			},
			{
				ID:          "bughunt-py-1",
				Language:    "python",
				Difficulty:  "medium",
				Title:       "Batas Perulangan Range Python",
				BuggyCode:   "total = 0\nfor i in range(1, 5):\n    total -= i\nprint(total)",
				BugLine:     &lineThree,
				Explanation: "PRIVATE_BUGHUNT_EXPLANATION_PY: Menggunakan operator pengurangan (-=) padahal tujuan mengakumulasi penjumlahan (+=).",
				XPReward:    70,
				Active:      true,
				Scope:       scope,
				Version:     1,
			},
		}
		for _, b := range bugs {
			if err = put(LearningBugHunt, b.ID, "", "active", "", b); err != nil {
				return err
			}
		}
		return nil
	})
}
