package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
)

type sourceTable struct {
	schema  string
	name    string
	pkCols  []string
	rowExpr string
}

// preservedRelations fixes the non-public relations that must survive the
// migration: credentials, SSO identities, and storage metadata.
var preservedRelations = [][2]string{
	{"auth", "users"},
	{"auth", "identities"},
	{"storage", "buckets"},
	{"storage", "objects"},
}

// runExtract copies every row of every public table of the legacy database into
// legacy_preserve.rows on the target, using one repeatable-read snapshot so the
// copy is internally consistent. Nothing is written back to the source.
func runExtract(ctx context.Context, cfg config) error {
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

	tables, err := listSourceTables(ctx, source)
	if err != nil {
		return err
	}
	log.Printf("extract: %d tables from source", len(tables))

	tx, err := target.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		create temp table _legacy_stage (
			source_table text, source_pk text, row_data jsonb
		) on commit drop`); err != nil {
		return err
	}

	snapshot, err := source.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return fmt.Errorf("source snapshot: %w", err)
	}
	defer snapshot.Rollback(ctx)

	counts := make(map[string]int, len(tables))
	for _, table := range tables {
		n, err := extractTable(ctx, snapshot, tx, table)
		if err != nil {
			return fmt.Errorf("extract %s.%s: %w", table.schema, table.name, err)
		}
		counts[table.schema+"."+table.name] = n
	}
	if err := snapshot.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return err
	}

	if _, err := tx.Exec(ctx, `
		insert into legacy_preserve.rows (source_table, source_pk, row_data, row_sha256, batch_id)
		select source_table, source_pk, row_data,
		       encode(sha256(convert_to(row_data::text, 'UTF8')), 'hex'), $1::text
		from _legacy_stage
		on conflict (source_table, source_pk) do update
		set row_data = excluded.row_data,
		    row_sha256 = excluded.row_sha256,
		    batch_id = excluded.batch_id`, cfg.batchID); err != nil {
		return err
	}

	fingerprint, err := preserveFingerprint(ctx, tx, "")
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into legacy_preserve.batches (batch_id, source_fingerprint, target_fingerprint, counts, status)
		values ($1::text, $2::text, '', $3::jsonb, 'planned')
		on conflict (batch_id) do update
		set source_fingerprint = excluded.source_fingerprint,
		    counts = excluded.counts,
		    status = case when legacy_preserve.batches.status = 'applied' then 'applied' else 'planned' end`,
		cfg.batchID, fingerprint, countsJSON(counts)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	total := 0
	for _, n := range counts {
		total += n
	}
	log.Printf("extract: preserved %d rows across %d tables (batch %s, fingerprint %s)",
		total, len(tables), cfg.batchID, truncate(fingerprint, 12))
	return nil
}

func listSourceTables(ctx context.Context, source interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) ([]sourceTable, error) {
	rows, err := source.Query(ctx, `
		select c.relname
		from pg_class c join pg_namespace n on n.oid = c.relnamespace
		where n.nspname = 'public' and c.relkind = 'r'
		order by c.relname`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	tables := make([]sourceTable, 0, len(names))
	for _, name := range names {
		pkCols, err := primaryKeyColumns(ctx, source, "public", name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, sourceTable{
			schema:  "public",
			name:    name,
			pkCols:  pkCols,
			rowExpr: rowExpression(pkCols),
		})
	}
	for _, relation := range preservedRelations {
		pkCols, err := primaryKeyColumns(ctx, source, relation[0], relation[1])
		if err != nil {
			return nil, err
		}
		tables = append(tables, sourceTable{
			schema:  relation[0],
			name:    relation[1],
			pkCols:  pkCols,
			rowExpr: rowExpression(pkCols),
		})
	}
	return tables, nil
}

func primaryKeyColumns(ctx context.Context, source interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, schema, table string) ([]string, error) {
	rows, err := source.Query(ctx, `
		select a.attname
		from pg_index i
		join pg_attribute a on a.attrelid = i.indrelid and a.attnum = any(i.indkey)
		where i.indrelid = format('%I.%I', $1::text, $2::text)::regclass and i.indisprimary
		order by array_position(i.indkey, a.attnum)`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

// rowExpression builds the primary-key text used as source_pk. Tables without a
// primary key fall back to the physical row order of the snapshot.
func rowExpression(pkCols []string) string {
	if len(pkCols) == 0 {
		return "row_number() over ()::text"
	}
	parts := make([]string, 0, len(pkCols))
	for _, col := range pkCols {
		parts = append(parts, fmt.Sprintf("coalesce(to_jsonb(t)->>%s, '<null>')", quoteLiteral(col)))
	}
	return "concat_ws('|', " + strings.Join(parts, ", ") + ")"
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func extractTable(ctx context.Context, snapshot, tx pgx.Tx, table sourceTable) (int, error) {
	query := fmt.Sprintf(`select %s, to_jsonb(t)::text from %s t`,
		table.rowExpr, pgx.Identifier{table.schema, table.name}.Sanitize())
	rows, err := snapshot.Query(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	records := make([][]any, 0, 1000)
	flush := func() error {
		if len(records) == 0 {
			return nil
		}
		copied, err := tx.CopyFrom(ctx, pgx.Identifier{"_legacy_stage"},
			[]string{"source_table", "source_pk", "row_data"},
			pgx.CopyFromRows(records))
		records = records[:0]
		if err != nil {
			return err
		}
		count += int(copied)
		return nil
	}
	for rows.Next() {
		var pk, data string
		if err := rows.Scan(&pk, &data); err != nil {
			return 0, err
		}
		records = append(records, []any{table.schema + "." + table.name, pk, data})
		if len(records) >= 1000 {
			if err := flush(); err != nil {
				return 0, err
			}
		}
	}
	if rows.Err() != nil {
		return 0, rows.Err()
	}
	if err := flush(); err != nil {
		return 0, err
	}
	return count, nil
}

func countsJSON(counts map[string]int) string {
	var b strings.Builder
	b.WriteString("{")
	first := true
	for table, n := range counts {
		if !first {
			b.WriteString(",")
		}
		first = false
		fmt.Fprintf(&b, "%q:%d", table, n)
	}
	b.WriteString("}")
	return b.String()
}
