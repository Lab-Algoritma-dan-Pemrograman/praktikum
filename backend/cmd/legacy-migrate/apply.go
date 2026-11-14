package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sort"

	"github.com/jackc/pgx/v5"
)

//go:embed sql/*.sql
var mappingFiles embed.FS

// runApply projects the preserved snapshot into the v2 domain tables. Everything
// runs inside a single transaction: either the whole projection lands or none of
// it does. Re-running the same batch is safe because every statement upserts by
// the legacy identifier.
func runApply(ctx context.Context, cfg config) error {
	target, err := openPool(ctx, cfg.targetURL, false)
	if err != nil {
		return fmt.Errorf("connect target: %w", err)
	}
	defer target.Close()

	var preserved int64
	if err := target.QueryRow(ctx, `select count(*) from legacy_preserve.rows`).Scan(&preserved); err != nil {
		return err
	}
	if preserved == 0 {
		return errors.New("legacy_preserve.rows is empty; run extract first")
	}

	statements, err := mappingStatements()
	if err != nil {
		return err
	}

	tx, err := target.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.body); err != nil {
			return fmt.Errorf("mapping %s: %w", statement.name, err)
		}
		log.Printf("apply: %s ok", statement.name)
	}

	fingerprint, err := domainFingerprint(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update legacy_preserve.batches
		set status = 'applied', target_fingerprint = $1, applied_at = now()
		where batch_id = $2`, fingerprint, cfg.batchID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Printf("apply: committed batch %s (domain fingerprint %s)", cfg.batchID, truncate(fingerprint, 12))
	return nil
}

type mappingStatement struct {
	name string
	body string
}

func mappingStatements() ([]mappingStatement, error) {
	names, err := fs.Glob(mappingFiles, "sql/*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	statements := make([]mappingStatement, 0, len(names))
	for _, name := range names {
		body, err := mappingFiles.ReadFile(name)
		if err != nil {
			return nil, err
		}
		statements = append(statements, mappingStatement{name: name, body: string(body)})
	}
	return statements, nil
}

// domainFingerprint summarises the migrated domain tables so two runs can be
// compared without dumping their content.
func domainFingerprint(ctx context.Context, tx pgx.Tx) (string, error) {
	rows, err := tx.Query(ctx, `
		select 'users', count(*) from public.users
		union all select 'user_role_assignments', count(*) from public.user_role_assignments
		union all select 'learning_levels', count(*) from public.learning_levels
		union all select 'learning_modules', count(*) from public.learning_modules
		union all select 'learning_lessons', count(*) from public.learning_lessons
		union all select 'learning_progress', count(*) from public.learning_progress
		union all select 'learning_profiles', count(*) from public.learning_profiles
		union all select 'learning_game_history', count(*) from public.learning_game_history
		union all select 'learning_bug_hunt', count(*) from public.learning_bug_hunt
		union all select 'practicum_sessions', count(*) from public.practicum_sessions
		union all select 'practicum_courses', count(*) from public.practicum_courses
		union all select 'practicum_questions', count(*) from public.practicum_questions
		union all select 'practicum_activations', count(*) from public.practicum_activations
		union all select 'practicum_attempts', count(*) from public.practicum_attempts
		union all select 'academic_classes', count(*) from public.academic_classes
		union all select 'academic_schedules', count(*) from public.academic_schedules
		union all select 'academic_attendance_records', count(*) from public.academic_attendance_records
		union all select 'academic_operations', count(*) from public.academic_operations
		union all select 'audit_logs', count(*) from public.audit_logs
		order by 1`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b []string
	for rows.Next() {
		var name string
		var n int64
		if err := rows.Scan(&name, &n); err != nil {
			return "", err
		}
		b = append(b, fmt.Sprintf("%s=%d", name, n))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return joinLines(b), nil
}
