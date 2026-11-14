package migrationrehearsal

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOwnedTargetGuardRejectsBeforeConnecting(t *testing.T) {
	if _, err := verifyTarget(context.Background(), nil); err == nil {
		t.Fatal("missing ownership accepted")
	}
	cases := []struct {
		name   string
		change func(*pgxpool.Config)
	}{
		{"remote", func(c *pgxpool.Config) { c.ConnConfig.Host = "remote.invalid" }},
		{"dns-loopback", func(c *pgxpool.Config) { c.ConnConfig.Host = "localhost" }},
		{"other-db", func(c *pgxpool.Config) { c.ConnConfig.Database = "other_test" }},
		{"other-port", func(c *pgxpool.Config) { c.ConnConfig.Port = 2 }},
		{"tls-fallback", func(c *pgxpool.Config) { c.ConnConfig.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12} }},
		{"fallback", func(c *pgxpool.Config) {
			c.ConnConfig.Fallbacks = []*pgconn.FallbackConfig{{Host: "remote.invalid", Port: 1}}
		}},
	}
	target := LocalTarget{RunID: "test-owned", Database: "owned_test", DataDirectory: "/owned", SystemIdentifier: "123", Port: 1}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := pgxpool.ParseConfig("host=127.0.0.1 port=1 dbname=owned_test user=phase6 password='' sslmode=disable")
			if err != nil {
				t.Fatal(err)
			}
			c.MinConns = 0
			c.ConnConfig.Fallbacks = nil
			tc.change(c)
			pool, err := pgxpool.NewWithConfig(t.Context(), c)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			if _, err = verifyTarget(WithLocalTarget(t.Context(), target), pool); err == nil {
				t.Fatal("unowned target accepted")
			}
		})
	}
}
func emptySnapshot() Snapshot {
	s := Snapshot{SchemaDigest: strings.Repeat("a", 64)}
	for _, name := range append(append([]string{}, publicTables...), "auth.users") {
		s.Tables = append(s.Tables, TableSnapshot{Name: name, Digest: digestValue([]string{})})
	}
	s.Digest = digestValue(s)
	return s
}
func TestReceiptAndSnapshotAccounting(t *testing.T) {
	if err := VerifyResult(nil); err == nil {
		t.Fatal("nil receipt accepted")
	}
	baseline := emptySnapshot()
	r := &Result{FormatVersion: FormatVersion, BatchID: "unit", Before: baseline, After: baseline, Counts: map[string]int{}}
	r.Digest = resultDigest(r)
	if err := VerifyResult(r); err != nil {
		t.Fatal(err)
	}
	r.Counts["rejected"] = 0
	r.Digest = resultDigest(r)
	if err := VerifyResult(r); err == nil {
		t.Fatal("unexpected accounting category accepted")
	}
	delete(r.Counts, "rejected")
	r.After.Tables = append([]TableSnapshot(nil), baseline.Tables...)
	r.After.Tables[0].Count = 1
	r.After.Digest = ""
	r.After.Digest = digestValue(r.After)
	r.Digest = resultDigest(r)
	if err := VerifyResult(r); err == nil {
		t.Fatal("unexplained extra target row accepted")
	}
}
func TestSnapshotCoversAllFieldsAndTables(t *testing.T) {
	s := emptySnapshot()
	s.Tables[0].Name = "lookalike"
	s.Digest = ""
	s.Digest = digestValue(s)
	if err := verifySnapshot(s); err == nil {
		t.Fatal("same-count wrong table set accepted")
	}
	s = emptySnapshot()
	s.Tables[0].Digest = strings.Repeat("b", 64)
	if err := verifySnapshot(s); err == nil {
		t.Fatal("changed table fingerprint accepted")
	}
	a, b := emptySnapshot(), emptySnapshot()
	b.Sequences = []SequenceSnapshot{{Name: "users_id_seq", LastValue: 123, IsCalled: true}}
	if !sameRows(a, b) {
		t.Fatal("row restoration should explicitly exclude sequence state")
	}
	if digestValue(a) == digestValue(b) {
		t.Fatal("exact restore fingerprint must include sequences")
	}
}
