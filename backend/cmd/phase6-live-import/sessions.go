package main

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

// acceptanceSessions only reads the rehearsal's one acceptance identity. It
// returns session IDs/state (not cookie values/hashes or provider credentials)
// so owned test sessions can be cleaned via the self-session API.
func acceptanceSessions(c *config, p *Plan) (map[string]any, error) {
	id := ""
	for _, u := range p.AuthUsers {
		if u.AcceptanceLogin {
			id = u.AuthUserID
		}
	}
	if id == "" {
		return nil, errors.New("acceptance identity missing")
	}
	tx, err := c.pool.BeginTx(c.ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer rollbackBounded(tx)
	if _, err = tx.Exec(c.ctx, `set local statement_timeout='10s'`); err != nil {
		return nil, err
	}
	rows, err := tx.Query(c.ctx, `select s.id,s.family_id,s.host,s.created_at,s.expires_at,s.revoked_at,
 (s.revoked_at is null and s.expires_at>now() and root.revoked_at is null and root.expires_at>now()) active
 from auth_sessions s join auth_sessions root on root.id=s.family_id
 join users u on u.id=s.user_id where u.auth_user_id=$1::uuid order by s.created_at`, id)
	if err != nil {
		return nil, errors.New("acceptance session metadata query failed")
	}
	sessions := []map[string]any{}
	active := 0
	for rows.Next() {
		var sid, family, host string
		var created, expires any
		var revoked any
		var a bool
		if err = rows.Scan(&sid, &family, &host, &created, &expires, &revoked, &a); err != nil {
			rows.Close()
			return nil, err
		}
		sessions = append(sessions, map[string]any{"id": sid, "family_id": family, "host": host, "created_at": created, "expires_at": expires, "revoked_at": revoked, "active": a})
		if a {
			active++
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	if err = tx.Commit(c.ctx); err != nil {
		return nil, err
	}
	return map[string]any{"active_sessions": active, "sessions": sessions, "scope": "phase6_single_acceptance_identity"}, nil
}
