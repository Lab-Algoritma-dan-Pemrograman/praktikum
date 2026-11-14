package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type tableFingerprint struct {
	table string
	rows  int64
	hash  string
}

// preserveFingerprint hashes the preserved copy so any later drift is visible.
func preserveFingerprint(ctx context.Context, tx pgx.Tx, batchID string) (string, error) {
	query := `
		select source_table, count(*), md5(coalesce(string_agg(row_sha256, '' order by row_sha256), ''))
		from legacy_preserve.rows
		where ($1::text = '' or batch_id = $1::text)
		group by source_table order by source_table`
	rows, err := tx.Query(ctx, query, batchID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var table, hash string
		var n int64
		if err := rows.Scan(&table, &n, &hash); err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%s:%d:%s", table, n, hash))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// runVerify re-reads the source inside a fresh read-only snapshot and compares
// it row by row against the preserved copy. This is the lossless proof.
func runVerify(ctx context.Context, cfg config) error {
	source, err := openPool(ctx, cfg.sourceURL, true)
	if err != nil {
		return fmt.Errorf("connect source: %w", err)
	}
	defer source.Close()
	target, err := openPool(ctx, cfg.targetURL, false)
	if err != nil {
		return fmt.Errorf("connect target: %w", err)
	}
	defer target.Close()

	snapshot, err := source.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer snapshot.Rollback(ctx)

	live, err := sourceFingerprints(ctx, snapshot)
	if err != nil {
		return err
	}
	preserved, err := preservedFingerprints(ctx, target)
	if err != nil {
		return err
	}

	index := make(map[string]tableFingerprint, len(preserved))
	for _, fp := range preserved {
		index[fp.table] = fp
	}
	var mismatches []string
	totalSource := int64(0)
	for _, fp := range live {
		totalSource += fp.rows
		got, ok := index[fp.table]
		if !ok {
			if fp.rows == 0 {
				continue
			}
			mismatches = append(mismatches, fmt.Sprintf("%s: missing in preserve", fp.table))
			continue
		}
		if got.rows != fp.rows || got.hash != fp.hash {
			mismatches = append(mismatches, fmt.Sprintf("%s: source %d/%s != preserve %d/%s",
				fp.table, fp.rows, truncate(fp.hash, 8), got.rows, truncate(got.hash, 8)))
		}
	}
	for _, fp := range preserved {
		found := false
		for _, liveFP := range live {
			if liveFP.table == fp.table {
				found = true
				break
			}
		}
		if !found {
			mismatches = append(mismatches, fmt.Sprintf("%s: exists in preserve only", fp.table))
		}
	}
	log.Printf("verify: %d source tables, %d source rows, %d mismatches", len(live), totalSource, len(mismatches))
	for _, m := range mismatches {
		log.Printf("  MISMATCH %s", m)
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("preserve verification failed: %d tables differ", len(mismatches))
	}
	return nil
}

func sourceFingerprints(ctx context.Context, snapshot pgx.Tx) ([]tableFingerprint, error) {
	relations, err := listSourceTables(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	result := make([]tableFingerprint, 0, len(relations))
	for _, relation := range relations {
		qualified := relation.schema + "." + relation.name
		query := fmt.Sprintf(`
			with rows as (
				select encode(sha256(convert_to(to_jsonb(t)::text, 'UTF8')), 'hex') as h from %s t
			)
			select count(*)::bigint, md5(coalesce(string_agg(h, '' order by h), '')) from rows`,
			pgx.Identifier{relation.schema, relation.name}.Sanitize())
		var fp tableFingerprint
		fp.table = qualified
		if err := snapshot.QueryRow(ctx, query).Scan(&fp.rows, &fp.hash); err != nil {
			return nil, fmt.Errorf("fingerprint %s: %w", qualified, err)
		}
		result = append(result, fp)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].table < result[j].table })
	return result, nil
}

func preservedFingerprints(ctx context.Context, target interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) ([]tableFingerprint, error) {
	rows, err := target.Query(ctx, `
		select source_table, count(*), md5(coalesce(string_agg(row_sha256, '' order by row_sha256), ''))
		from legacy_preserve.rows group by source_table order by source_table`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []tableFingerprint
	for rows.Next() {
		var fp tableFingerprint
		if err := rows.Scan(&fp.table, &fp.rows, &fp.hash); err != nil {
			return nil, err
		}
		result = append(result, fp)
	}
	return result, rows.Err()
}
