package main

import (
	"encoding/json"
	"testing"
)

func TestBindValueRequiresDeclaredReferences(t *testing.T) {
	users := map[string]int64{"u": 42}
	ids := map[string]string{"p": "parent"}
	got, err := bindValue(map[string]any{"user": map[string]any{"$user": "u"}, "parent": map[string]any{"$id": "p"}, "text": "$user"}, users, ids)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(got)
	if string(b) != `{"parent":"parent","text":"$user","user":42}` {
		t.Fatalf("unexpected bind %s", b)
	}
	if _, err = bindValue(map[string]any{"$user": "missing"}, users, ids); err == nil {
		t.Fatal("dangling user accepted")
	}
	if _, err = bindValue(map[string]any{"$id": "missing"}, users, ids); err == nil {
		t.Fatal("dangling ID accepted")
	}
}
func TestValidatePlanAccountingAndTables(t *testing.T) {
	p := Plan{FormatVersion: format, BatchID: "b", AuthUsers: []AuthUser{{Token: "u", AuthUserID: "uuid", Email: "x@example.test"}}, Users: []User{{Token: "u", AuthUserID: "uuid", Email: "x@example.test"}}, Assignments: []Assignment{{Token: "a", UserToken: "u"}}, DomainRows: []DomainRow{{Token: "d", Table: "learning_levels", ID: "id"}}, Dispositions: []Disposition{{SourceSystem: "s", Entity: "e", SourceRef: "1", Disposition: "imported", Reason: "ok", Rows: 1}}, Counts: map[string]int{"imported": 1, "rejected": 0}}
	if err := validatePlan(&p); err != nil {
		t.Fatal(err)
	}
	p.DomainRows[0].Table = "audit_logs"
	if err := validatePlan(&p); err == nil {
		t.Fatal("unapproved table accepted")
	}
	p.DomainRows[0].Table = "learning_levels"
	p.Counts["imported"] = 2
	if err := validatePlan(&p); err == nil {
		t.Fatal("bad accounting accepted")
	}
}
func TestReceiptTableIsExplicitAndAllowlisted(t *testing.T) {
	for _, c := range []struct{ kind, table, want string }{
		{"auth_user", "auth.users", `"auth"."users"`},
		{"user", "public.users", `"public"."users"`},
		{"assignment", "public.user_role_assignments", `"public"."user_role_assignments"`},
		{"domain", "learning_progress", `"public"."learning_progress"`},
	} {
		got, err := receiptTable(c.kind, c.table)
		if err != nil || got.Sanitize() != c.want {
			t.Fatalf("wrong table resolution for %s/%s", c.kind, c.table)
		}
	}
	for _, c := range []struct{ kind, table string }{
		{"user", "users"}, {"domain", "auth.users"}, {"domain", "public.learning_progress"},
		{"assignment", "public.users"}, {"unknown", "public.users"}, {"auth_user", "public.users"},
	} {
		if _, err := receiptTable(c.kind, c.table); err == nil {
			t.Fatalf("unexpected table accepted: %s/%s", c.kind, c.table)
		}
	}
}

func TestReceiptDigestBinding(t *testing.T) {
	p := Plan{BatchID: "b", AuthUsers: []AuthUser{}, Users: []User{}, Assignments: []Assignment{}, DomainRows: []DomainRow{}}
	r := Receipt{FormatVersion: "phase6-live-receipt-v1", BatchID: "b", PlanFileSHA256: "h", TargetFingerprint: "t", Counts: map[string]int{}}
	r.Digest = digest(&r)
	if err := verifyReceipt(&r, &p, "h", "t"); err != nil {
		t.Fatal(err)
	}
	r.TargetFingerprint = "other"
	if err := verifyReceipt(&r, &p, "h", "t"); err == nil {
		t.Fatal("wrong target receipt accepted")
	}
}
