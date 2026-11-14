package httpdelivery

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
)

func TestRevokingIssuerInvalidatesPendingSSOHandshake(t *testing.T) {
	h := newAuthHarness(t, 120)
	central := h.login(t, "koordinator", authHost)
	first := h.request("POST", "/v1/auth/sso/exchange", exchangeBody(h.prepare(t, central, adminHost), adminHost, verifier), adminHost, nil, false)
	requireStatus(t, first, 200)
	child := first.Result().Cookies()
	sum := sha256.Sum256([]byte(verifier))
	target := "siakad.algohub.web.id"
	pending := h.request("POST", "/v1/auth/sso/prepare", fmt.Sprintf(`{"return_to":"https://%s/","code_challenge":"%s"}`, target, base64.RawURLEncoding.EncodeToString(sum[:])), adminHost, child, true)
	requireStatus(t, pending, 200)
	code := responseData[struct {
		Code string `json:"code"`
	}](t, pending).Code
	session, err := h.repo.FindSessionByTokenHash(t.Context(), security.Hash(child[0].Value))
	if err != nil {
		t.Fatal(err)
	}
	requireStatus(t, h.request("DELETE", "/v1/auth/sessions/"+session.ID, "", authHost, central, true), 200)
	requireStatus(t, h.request("POST", "/v1/auth/sso/exchange", exchangeBody(code, target, verifier), target, nil, false), 401)
	requireStatus(t, h.request("GET", "/v1/auth/me", "", authHost, central, false), 200)
}

func TestConcurrentRefreshHasOneWinner(t *testing.T) {
	h := newAuthHarness(t, 120)
	cookies := h.login(t, "koordinator", authHost)
	results := make(chan int, 8)
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- h.request("POST", "/v1/auth/refresh", "", authHost, cookies, true).Code
		}()
	}
	group.Wait()
	close(results)
	winners := 0
	for status := range results {
		switch status {
		case 200:
			winners++
		case 401, 403:
		default:
			t.Fatalf("unexpected refresh status %d", status)
		}
	}
	if winners != 1 {
		t.Fatalf("want one rotation, got %d", winners)
	}
}

func TestLogoutRacingExchangeCannotLeaveActiveChild(t *testing.T) {
	h := newAuthHarness(t, 120)
	central := h.login(t, "koordinator", authHost)
	body := exchangeBody(h.prepare(t, central, adminHost), adminHost, verifier)
	var child []*http.Cookie
	var exchangeStatus, logoutStatus int
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		result := h.request("POST", "/v1/auth/sso/exchange", body, adminHost, nil, false)
		exchangeStatus, child = result.Code, result.Result().Cookies()
	}()
	go func() {
		defer group.Done()
		logoutStatus = h.request("POST", "/v1/auth/logout", "", authHost, central, true).Code
	}()
	group.Wait()
	if logoutStatus != 200 || (exchangeStatus != 200 && exchangeStatus != 401) {
		t.Fatalf("unexpected race outcome logout=%d exchange=%d", logoutStatus, exchangeStatus)
	}
	if child != nil {
		requireStatus(t, h.request("GET", "/v1/auth/me", "", adminHost, child, false), 401)
	}
}
