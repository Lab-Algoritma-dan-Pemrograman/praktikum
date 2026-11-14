package httpdelivery

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

func TestScopedAuditNeverLeaksOtherClassAndAdvancesEmptyPages(t *testing.T) {
	h := newAuthHarness(t, 120)
	coordinator := h.login(t, "koordinator", adminHost)
	scope := entity.Scope{Domain: "praktikum", Division: "pendidikan", ClassID: "class-101", Shift: 1, SessionID: "session-1"}
	body := `{"version":1,"assignments":[{"role":"asisten","division":"pendidikan","scope":{"domain":"praktikum","division":"pendidikan","class_id":"class-101","shift":1,"session_id":"session-1"}}],"grants":[{"permission":"audit.read","role":"asisten","division":"pendidikan","scope":{"domain":"praktikum","division":"pendidikan","class_id":"class-101","shift":1,"session_id":"session-1"}}]}`
	requireStatus(t, h.request("PUT", "/v1/admin/users/11/access", body, adminHost, coordinator, true), 200)
	staff := h.login(t, "scoped", adminHost)
	for _, classID := range []string{"class-101", "class-202"} {
		eventScope := scope
		eventScope.ClassID = classID
		if err := h.repo.AppendAudit(t.Context(), entity.AuditEvent{Action: "test.scope", App: "identity", SubjectType: "fixture", SubjectID: classID, Scope: eventScope, RequestID: "req_scope", OperationID: "test.scope", CreatedAt: h.now}); err != nil {
			t.Fatal(err)
		}
	}
	first := h.request("GET", "/v1/admin/auditlog?limit=1", "", adminHost, staff, false)
	requireStatus(t, first, 200)
	if len(responseData[[]entity.AuditEvent](t, first)) != 0 {
		t.Fatal("out-of-scope event was exposed")
	}
	var page struct {
		Meta struct {
			Next int64 `json:"next_before"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil || page.Meta.Next == 0 {
		t.Fatal("empty filtered page lost its scan cursor")
	}
	second := h.request("GET", fmt.Sprintf("/v1/admin/auditlog?limit=1&before=%d", page.Meta.Next), "", adminHost, staff, false)
	requireStatus(t, second, 200)
	items := responseData[[]entity.AuditEvent](t, second)
	if len(items) != 1 || items[0].SubjectID != "class-101" {
		t.Fatal("authorized event not available after filtered page")
	}
}

func TestInactiveAccountCannotLoginOrUseExistingSession(t *testing.T) {
	h := newAuthHarness(t, 120)
	cookies := h.login(t, "mahasiswa", authHost)
	user, err := h.repo.FindUserByID(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	user.Active = false
	if err := h.repo.SaveUser(t.Context(), user); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, h.request("GET", "/v1/auth/me", "", authHost, cookies, false), 403)
	requireStatus(t, h.request("POST", "/v1/auth/password", `{"email":"mahasiswa@example.test","password":"fixture-password"}`, authHost, nil, false), 403)
}
