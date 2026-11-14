package main

import (
	"errors"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

// seedLocalAuthMetadata restores only non-credential Auth prerequisites on an
// owned local recovery database. It never operates on a remote target and never
// changes existing provider identities. Password hashes are not restored here.
func seedLocalAuthMetadata(c *config, path string) (map[string]any, error) {
	if c.local == nil {
		return nil, errors.New("Auth metadata recovery is local-only")
	}
	var items []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Batch string `json:"batch"`
	}
	if err := readStrict(path, 2<<20, &items); err != nil {
		return nil, err
	}
	tx, err := c.pool.BeginTx(c.ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer rollbackBounded(tx)
	if _, err = tx.Exec(c.ctx, `set local statement_timeout='30s';set local lock_timeout='5s'`); err != nil {
		return nil, err
	}
	for _, u := range items {
		if u.ID == "" || u.Email == "" {
			return nil, errors.New("invalid Auth recovery metadata")
		}
		_, err = tx.Exec(c.ctx, `insert into auth.users(id,email,raw_user_meta_data) values($1::uuid,$2::text,case when $3::text='' then null else jsonb_build_object('phase6_batch',$3::text) end)`, u.ID, u.Email, u.Batch)
		if err != nil {
			return nil, safePG("Auth metadata recovery failed", err)
		}
	}
	if err = tx.Commit(c.ctx); err != nil {
		return nil, err
	}
	return map[string]any{"status": "passed", "seeded_auth_metadata": len(items)}, nil
}

// setLocalAcceptancePassword generates only the test credential on a restored
// owned local copy. It does not change remote passwords or any imported public
// row. The real staging credential stays in its original private artifact.
func setLocalAcceptancePassword(c *config, p *Plan) (map[string]any, error) {
	if c.local == nil {
		return nil, errors.New("test password recovery is local-only")
	}
	password := os.Getenv("PHASE6_ACCEPTANCE_PASSWORD")
	if len(password) < 16 {
		return nil, errors.New("private acceptance password missing")
	}
	id := ""
	for _, u := range p.AuthUsers {
		if u.AcceptanceLogin {
			if id != "" {
				return nil, errors.New("multiple acceptance identities")
			}
			id = u.AuthUserID
		}
	}
	if id == "" {
		return nil, errors.New("acceptance identity missing")
	}
	tag, err := c.pool.Exec(c.ctx, `update auth.users set encrypted_password=crypt($2::text,gen_salt('bf')) where id=$1::uuid`, id, password)
	if err != nil || tag.RowsAffected() != 1 {
		return nil, errors.New("local acceptance password recovery failed")
	}
	return map[string]any{"status": "passed", "credential_scope": "owned_local_copy_only"}, nil
}

func exportAuthMetadata(c *config) ([]map[string]string, error) {
	tx, err := c.pool.BeginTx(c.ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer rollbackBounded(tx)
	if _, err = tx.Exec(c.ctx, `set local statement_timeout='10s'`); err != nil {
		return nil, err
	}
	rows, err := tx.Query(c.ctx, `select id::text,email,coalesce(raw_user_meta_data->>'phase6_batch','') from auth.users order by id`)
	if err != nil {
		return nil, errors.New("Auth metadata read failed")
	}
	result := []map[string]string{}
	for rows.Next() {
		var id, email, batch string
		if err = rows.Scan(&id, &email, &batch); err != nil {
			rows.Close()
			return nil, err
		}
		if !strings.HasSuffix(email, ".test") {
			rows.Close()
			return nil, errors.New("Auth metadata export restricted to synthetic test identities")
		}
		result = append(result, map[string]string{"id": id, "email": email, "batch": batch})
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	if err = tx.Commit(c.ctx); err != nil {
		return nil, err
	}
	return result, nil
}
