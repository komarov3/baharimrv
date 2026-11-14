package app_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
)

// loginWithRefresh signs a fresh account in and returns the access token plus
// the refresh grant the server issued alongside it.
func (f *fixture) loginWithRefresh(local string) (string, string) {
	f.t.Helper()
	email := local + "@test.baharimrv.local"
	f.must(http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"email": email, "password": testPassword, "displayName": local, "role": models.RoleManager,
	}, http.StatusCreated)
	login := f.must(http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email": email, "password": testPassword,
	}, http.StatusOK)
	refresh := login.str("refreshToken")
	if refresh == "" {
		f.t.Fatalf("login did not issue a refresh grant: %s", login.raw)
	}
	return login.str("sessionToken"), refresh
}

func TestRefreshRotatesGrantAndRenewsSession(t *testing.T) {
	f := newFixture(t)
	_, refresh := f.loginWithRefresh("rotate")

	first := f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": refresh}, http.StatusOK)
	newToken := first.str("sessionToken")
	newRefresh := first.str("refreshToken")
	if newToken == "" || newRefresh == "" || newRefresh == refresh {
		t.Fatalf("refresh must return a new session and a rotated grant: %s", first.raw)
	}
	f.must(http.MethodGet, "/api/v1/auth/me", newToken, nil, http.StatusOK)

	// The rotated grant continues the chain.
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": newRefresh}, http.StatusOK)
	// The spent grant is dead even though it is unexpired. Presenting it last,
	// because rejection also revokes the whole family.
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": refresh}, http.StatusUnauthorized)
}

func TestRefreshReuseRevokesTheFamily(t *testing.T) {
	f := newFixture(t)
	_, refresh := f.loginWithRefresh("reuse")

	first := f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": refresh}, http.StatusOK)
	rotated := first.str("refreshToken")

	// Presenting the spent grant again is a theft signal: every grant for the
	// user dies, including the rotated one the legitimate client holds.
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": refresh}, http.StatusUnauthorized)
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": rotated}, http.StatusUnauthorized)
}

func TestRefreshRejectsUnknownAndEmptyGrants(t *testing.T) {
	f := newFixture(t)
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": "not-a-grant"}, http.StatusUnauthorized)
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": ""}, http.StatusUnauthorized)
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{}, http.StatusUnauthorized)
}

func TestRefreshRejectsExpiredGrant(t *testing.T) {
	f := newFixture(t)
	_, refresh := f.loginWithRefresh("expired")
	if err := f.app.DB.Model(&models.RefreshGrant{}).
		Where("token_hash <> ''").
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire the grant: %v", err)
	}
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": refresh}, http.StatusUnauthorized)
}

func TestLogoutRevokesRefreshGrants(t *testing.T) {
	f := newFixture(t)
	token, refresh := f.loginWithRefresh("logout")
	f.must(http.MethodPost, "/api/v1/auth/logout", token, nil, http.StatusNoContent)
	f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": refresh}, http.StatusUnauthorized)
}

func TestRefreshKeepsWorkingAfterAccessSessionExpires(t *testing.T) {
	f := newFixture(t)
	token, refresh := f.loginWithRefresh("idle")
	if err := f.app.DB.Model(&models.LocalSession{}).
		Where("token_hash <> ''").
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire the session: %v", err)
	}
	f.must(http.MethodGet, "/api/v1/auth/me", token, nil, http.StatusUnauthorized)
	renewed := f.must(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": refresh}, http.StatusOK)
	f.must(http.MethodGet, "/api/v1/auth/me", renewed.str("sessionToken"), nil, http.StatusOK)
}