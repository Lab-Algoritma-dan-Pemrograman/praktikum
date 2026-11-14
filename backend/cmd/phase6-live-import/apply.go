package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type boundDomain struct {
	row         DomainRow
	id, parent  string
	owner       int64
	scope, data []byte
	unique      string
}

func apply(c *config, p *Plan, fileHash string) (*Receipt, error) {
	started := time.Now().UTC()
	tx, err := c.pool.BeginTx(c.ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer rollbackBounded(tx)
	if _, err = tx.Exec(c.ctx, `set local statement_timeout='15min';set local lock_timeout='10s';set local idle_in_transaction_session_timeout='16min';select pg_advisory_xact_lock(5784123647119155286)`); err != nil {
		return nil, err
	}
	receipt := &Receipt{FormatVersion: "phase6-live-receipt-v1", BatchID: p.BatchID, PlanFileSHA256: fileHash, TargetFingerprint: c.fingerprint, Counts: map[string]int{}, StartedAt: started}
	userIDs := map[string]int64{}
	keys := map[string]string{}
	for _, r := range p.DomainRows {
		if r.ID != "" {
			keys[r.Token] = r.ID
		}
	}
	fullAuth := false
	if err = tx.QueryRow(c.ctx, `select exists(select 1 from information_schema.columns where table_schema='auth' and table_name='users' and column_name='email')`).Scan(&fullAuth); err != nil {
		return nil, err
	}
	password := os.Getenv("PHASE6_ACCEPTANCE_PASSWORD")
	acceptance := 0
	for _, u := range p.AuthUsers {
		if u.AcceptanceLogin {
			acceptance++
			if fullAuth && len(password) < 16 {
				return nil, errors.New("acceptance password must be private and at least 16 characters")
			}
		}
		existed, err := authExists(c.ctx, tx, u.AuthUserID)
		if err != nil {
			return nil, err
		}
		if !existed {
			if fullAuth {
				hashExpr := `''`
				args := []any{u.AuthUserID, u.Email, u.Name, u.CreatedAt}
				if u.AcceptanceLogin {
					hashExpr = `crypt($5::text,gen_salt('bf'))`
					args = append(args, password)
				}
				query := `insert into auth.users(instance_id,id,aud,role,email,encrypted_password,email_confirmed_at,confirmation_token,recovery_token,email_change_token_new,email_change,raw_app_meta_data,raw_user_meta_data,created_at,updated_at) values('00000000-0000-0000-0000-000000000000',$1::uuid,'authenticated','authenticated',$2::text,` + hashExpr + `,$4::timestamptz,'','','','',jsonb_build_object('provider','email','providers',jsonb_build_array('email')),jsonb_build_object('name',$3::text,'phase6_batch','` + p.BatchID + `'),$4::timestamptz,$4::timestamptz)`
				if _, err = tx.Exec(c.ctx, query, args...); err != nil {
					return nil, safePG("auth user insert failed", err)
				}
				if _, err = tx.Exec(c.ctx, `insert into auth.identities(provider_id,user_id,identity_data,provider,last_sign_in_at,created_at,updated_at) values($1::text,$2::uuid,jsonb_build_object('sub',$2::text,'email',$1::text,'email_verified',true),'email',$3::timestamptz,$3::timestamptz,$3::timestamptz) on conflict(provider_id,provider) do nothing`, u.Email, u.AuthUserID, u.CreatedAt); err != nil {
					return nil, errors.New("auth identity insert failed")
				}
			} else {
				if _, err = tx.Exec(c.ctx, `insert into auth.users(id) values($1)`, u.AuthUserID); err != nil {
					return nil, err
				}
			}
		}
		var email *string
		if fullAuth {
			if err = tx.QueryRow(c.ctx, `select email from auth.users where id=$1`, u.AuthUserID).Scan(&email); err != nil || email == nil || *email != u.Email {
				return nil, errors.New("auth user conflict")
			}
		}
		dg, err := rowDigest(c.ctx, tx, "auth.users", "id", u.AuthUserID)
		if err != nil {
			return nil, err
		}
		addReceipt(receipt, ReceiptRow{Token: u.Token, Kind: "auth_user", Table: "auth.users", Key: u.AuthUserID, Action: action(existed), Digest: dg})
	}
	if fullAuth && acceptance != 1 {
		return nil, errors.New("exactly one acceptance identity required")
	}
	for _, u := range p.Users {
		existed := false
		var id int64
		var nim *string
		var email, name string
		var active, hidden bool
		var created time.Time
		err = tx.QueryRow(c.ctx, `select id,nim,email,name,is_active,hidden,created_at from public.users where auth_user_id=$1`, u.AuthUserID).Scan(&id, &nim, &email, &name, &active, &hidden, &created)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(c.ctx, `insert into public.users(auth_user_id,nim,email,name,is_active,hidden,created_at,updated_at,access_version) values($1,$2,$3,$4,$5,$6,$7,$7,1) returning id`, u.AuthUserID, u.NIM, u.Email, u.Name, u.IsActive, u.Hidden, u.CreatedAt).Scan(&id)
			if err == nil {
				err = tx.QueryRow(c.ctx, `select nim,email,name,is_active,hidden,created_at from public.users where id=$1`, id).Scan(&nim, &email, &name, &active, &hidden, &created)
			}
		} else if err == nil {
			existed = true
		} else {
			return nil, err
		}
		if !equalPtr(nim, u.NIM) || email != u.Email || name != u.Name || active != u.IsActive || hidden != u.Hidden || !created.Equal(u.CreatedAt) {
			return nil, errors.New("public user conflict")
		}
		userIDs[u.Token] = id
		dg, err := rowDigest(c.ctx, tx, "public.users", "id", id)
		if err != nil {
			return nil, err
		}
		addReceipt(receipt, ReceiptRow{Token: u.Token, Kind: "user", Table: "public.users", Key: strconv.FormatInt(id, 10), Action: action(existed), Digest: dg})
	}
	for _, a := range p.Assignments {
		uid, ok := userIDs[a.UserToken]
		if !ok {
			return nil, errors.New("assignment user unresolved")
		}
		scope, err := bindJSON(a.Scope, userIDs, keys)
		if err != nil {
			return nil, err
		}
		var id int64
		var active bool
		var created time.Time
		existed := false
		err = tx.QueryRow(c.ctx, `select id,is_active,created_at from public.user_role_assignments where user_id=$1 and role_code=$2 and division_code is not distinct from $3 and scope=$4`, uid, a.RoleCode, a.DivisionCode, json.RawMessage(scope)).Scan(&id, &active, &created)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(c.ctx, `insert into public.user_role_assignments(user_id,role_code,division_code,scope,is_active,created_at) values($1,$2,$3,$4,$5,$6) returning id`, uid, a.RoleCode, a.DivisionCode, json.RawMessage(scope), a.IsActive, a.CreatedAt).Scan(&id)
			if err == nil {
				err = tx.QueryRow(c.ctx, `select is_active,created_at from public.user_role_assignments where id=$1`, id).Scan(&active, &created)
			}
		} else if err == nil {
			existed = true
		} else {
			return nil, err
		}
		if active != a.IsActive || !created.Equal(a.CreatedAt) {
			return nil, errors.New("assignment conflict")
		}
		if !existed {
			// Effective permissions are explicit grant rows, not a live join on
			// the role catalogs. Materialize the canonical defaults exactly once
			// for each newly inserted assignment so imported identities behave
			// like normally provisioned accounts.
			if _, err = tx.Exec(c.ctx, `
				insert into public.user_permission_grants(user_id,assignment_id,permission_code,scope,is_active,created_at)
				select $1::bigint,$2::bigint,x.permission_code,x.scope,true,$3::timestamptz
				from (select distinct on (permission_code) permission_code,scope
				      from (select permission_code,default_scope scope,1 pri from public.role_permissions where role_code=$4
				            union all
				            select permission_code,default_scope,2 from public.division_permissions where $5::text is not null and division_code=$5)
				      candidates order by permission_code,pri) x
				on conflict do nothing`, uid, id, a.CreatedAt, a.RoleCode, a.DivisionCode); err != nil {
				return nil, safePG("permission grant materialization failed", err)
			}
		}
		dg, err := rowDigest(c.ctx, tx, "public.user_role_assignments", "id", id)
		if err != nil {
			return nil, err
		}
		addReceipt(receipt, ReceiptRow{Token: a.Token, Kind: "assignment", Table: "public.user_role_assignments", Key: strconv.FormatInt(id, 10), Action: action(existed), Digest: dg})
	}
	bound := make([]boundDomain, 0, len(p.DomainRows))
	for _, r := range p.DomainRows {
		parent := ""
		if r.ParentID != "" {
			var ok bool
			parent, ok = keys[r.ParentID]
			if !ok {
				return nil, fmt.Errorf("parent unresolved for %s", r.Token)
			}
		}
		owner := int64(0)
		if r.OwnerToken != "" {
			var ok bool
			owner, ok = userIDs[r.OwnerToken]
			if !ok {
				return nil, errors.New("domain owner unresolved")
			}
		}
		id := r.ID
		switch r.IDTemplate {
		case "profile":
			id = "profile-" + strconv.FormatInt(owner, 10)
		case "progress":
			id = "progress-" + parent + "-" + strconv.FormatInt(owner, 10)
		case "":
		default:
			return nil, errors.New("unknown ID template")
		}
		if id == "" || len(id) > 128 {
			return nil, errors.New("bound ID invalid")
		}
		keys[r.Token] = id
		scope, err := bindJSON(r.Scope, userIDs, keys)
		if err != nil {
			return nil, err
		}
		data, err := bindJSON(r.Data, userIDs, keys)
		if err != nil {
			return nil, err
		}
		// Freeze triggers compare jsonb documents. Persist the exact document a
		// normal application read/write round-trip would produce, otherwise an
		// omitted-empty optional field would look like a mutation later.
		scope, data, err = normalizeForTable(r.Table, scope, data)
		if err != nil {
			return nil, fmt.Errorf("payload is not round-trip stable for %s", r.Table)
		}
		unique := r.UniqueKey
		if r.IDTemplate == "profile" {
			unique = id
		}
		if r.IDTemplate == "progress" || r.Table == "practicum_attempts" {
			unique = parent + ":" + strconv.FormatInt(owner, 10)
		}
		bound = append(bound, boundDomain{r, id, parent, owner, scope, data, unique})
	}
	// Preserve dependency order while pipelining each contiguous table group.
	for start := 0; start < len(bound); {
		end := start + 1
		for end < len(bound) && bound[end].row.Table == bound[start].row.Table {
			end++
		}
		if err = applyDomainGroup(c.ctx, tx, bound[start:end], receipt); err != nil {
			return nil, err
		}
		start = end
	}
	if err = tx.Commit(c.ctx); err != nil {
		return nil, err
	}
	receipt.FinishedAt = time.Now().UTC()
	receipt.DurationMS = receipt.FinishedAt.Sub(started).Milliseconds()
	receipt.Digest = ""
	receipt.Digest = digest(receipt)
	return receipt, nil
}
func applyDomainGroup(ctx context.Context, tx pgx.Tx, group []boundDomain, receipt *Receipt) error {
	table := group[0].row.Table
	if !allowedTables[table] {
		return errors.New("table not allowed")
	}
	ids := make([]string, len(group))
	for i, r := range group {
		ids[i] = r.id
	}
	existing := map[string]string{}
	rows, err := tx.Query(ctx, `select id,to_jsonb(t)::text from `+pgx.Identifier{"public", table}.Sanitize()+` t where id=any($1)`, ids)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, body string
		if err = rows.Scan(&id, &body); err != nil {
			rows.Close()
			return err
		}
		existing[id] = digest(json.RawMessage(body))
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	for base := 0; base < len(group); base += 500 {
		limit := base + 500
		if limit > len(group) {
			limit = len(group)
		}
		b := &pgx.Batch{}
		for _, r := range group[base:limit] {
			b.Queue(`insert into `+pgx.Identifier{"public", table}.Sanitize()+`(id,parent_id,owner_id,scope,status,unique_key,expires_at,data) values($1,$2,$3,$4,$5,$6,$7,$8) on conflict(id) do nothing`, r.id, r.parent, r.owner, json.RawMessage(r.scope), r.row.Status, r.unique, r.row.ExpiresAt, json.RawMessage(r.data))
		}
		br := tx.SendBatch(ctx, b)
		for range group[base:limit] {
			if _, err = br.Exec(); err != nil {
				br.Close()
				return errors.New("domain batch insert failed for " + table)
			}
		}
		if err = br.Close(); err != nil {
			return err
		}
	}
	rows, err = tx.Query(ctx, `select id,to_jsonb(t)::text from `+pgx.Identifier{"public", table}.Sanitize()+` t where id=any($1)`, ids)
	if err != nil {
		return err
	}
	actual := map[string]string{}
	for rows.Next() {
		var id, body string
		if err = rows.Scan(&id, &body); err != nil {
			rows.Close()
			return err
		}
		actual[id] = digest(json.RawMessage(body))
	}
	rows.Close()
	if len(actual) != len(group) {
		return errors.New("domain insert count mismatch")
	}
	expected := make([]map[string]any, 0, len(group))
	for _, r := range group {
		expected = append(expected, map[string]any{"id": r.id, "parent_id": r.parent, "owner_id": r.owner, "scope": json.RawMessage(r.scope), "status": r.row.Status, "unique_key": r.unique, "expires_at": r.row.ExpiresAt, "data": json.RawMessage(r.data)})
	}
	expectedJSON, _ := json.Marshal(expected)
	var mismatches int
	compare := `with expected as (select * from jsonb_to_recordset($1::jsonb) as e(id text,parent_id text,owner_id bigint,scope jsonb,status text,unique_key text,expires_at timestamptz,data jsonb)) select count(*) from expected e left join ` + pgx.Identifier{"public", table}.Sanitize() + ` t on t.id=e.id where t.id is null or (t.parent_id,t.owner_id,t.scope,t.status,t.unique_key,t.expires_at,t.data) is distinct from (e.parent_id,e.owner_id,e.scope,e.status,e.unique_key,e.expires_at,e.data)`
	if err = tx.QueryRow(ctx, compare, string(expectedJSON)).Scan(&mismatches); err != nil || mismatches != 0 {
		return errors.New("domain row differs from deterministic plan: " + table)
	}
	for _, r := range group {
		dg := actual[r.id]
		if prior, ok := existing[r.id]; ok && prior != dg {
			return errors.New("existing domain row changed")
		}
		addReceipt(receipt, ReceiptRow{Token: r.row.Token, Kind: "domain", Table: table, Key: r.id, Action: action(existing[r.id] != ""), Digest: dg})
	}
	return nil
}
func bindJSON(v any, userIDs map[string]int64, keys map[string]string) ([]byte, error) {
	bound, err := bindValue(v, userIDs, keys)
	if err != nil {
		return nil, err
	}
	return json.Marshal(bound)
}
func bindValue(v any, userIDs map[string]int64, keys map[string]string) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 1 {
			if t, ok := x["$user"].(string); ok {
				id, yes := userIDs[t]
				if !yes {
					return nil, errors.New("user marker unresolved")
				}
				return id, nil
			}
			if t, ok := x["$id"].(string); ok {
				id, yes := keys[t]
				if !yes {
					return nil, errors.New("ID marker unresolved")
				}
				return id, nil
			}
		}
		out := map[string]any{}
		for k, val := range x {
			b, e := bindValue(val, userIDs, keys)
			if e != nil {
				return nil, e
			}
			out[k] = b
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			b, e := bindValue(val, userIDs, keys)
			if e != nil {
				return nil, e
			}
			out[i] = b
		}
		return out, nil
	default:
		return v, nil
	}
}
func authExists(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `select exists(select 1 from auth.users where id=$1)`, id).Scan(&ok)
	return ok, err
}
func rowDigest(ctx context.Context, tx pgx.Tx, table, col string, key any) (string, error) {
	var body string
	err := tx.QueryRow(ctx, `select to_jsonb(t)::text from `+pgx.Identifier(stringsSplit(table)).Sanitize()+` t where `+pgx.Identifier{col}.Sanitize()+`=$1`, key).Scan(&body)
	if err != nil {
		return "", err
	}
	return digest(json.RawMessage(body)), nil
}
func stringsSplit(s string) []string {
	var out []string
	part := ""
	for _, r := range s {
		if r == '.' {
			out = append(out, part)
			part = ""
		} else {
			part += string(r)
		}
	}
	return append(out, part)
}
func addReceipt(r *Receipt, row ReceiptRow) { r.Rows = append(r.Rows, row); r.Counts[row.Action]++ }
func action(existed bool) string {
	if existed {
		return "unchanged"
	}
	return "inserted"
}
func equalPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// safePG reports only schema-level diagnostics, never row values.
func safePG(msg string, err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return fmt.Errorf("%s [%s constraint=%s column=%s table=%s]", msg, pg.Code, pg.ConstraintName, pg.ColumnName, pg.TableName)
	}
	return errors.New(msg)
}
func sortReceipt(rows []ReceiptRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Token < rows[j].Token })
}
