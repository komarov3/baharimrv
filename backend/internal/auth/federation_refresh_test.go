package auth

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/chain"
	"github.com/baharilink/baharimrv/internal/database"
	"github.com/baharilink/baharimrv/internal/identity"
)

type stubValidator struct {
	claims ExternalClaims
	err    error
}

func (s stubValidator) Validate(context.Context, string, string, string) (ExternalClaims, error) {
	return s.claims, s.err
}

func (s stubValidator) ExchangeCode(context.Context, string, string, string, string) (string, error) {
	return "", fmt.Errorf("not used in this test")
}

func TestFederatedLoginIssuesAUsableRefreshGrant(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "federation.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := database.Open(dsn, false)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	kek := make([]byte, 32)
	custody, err := identity.NewEnvelopeCustody(kek)
	if err != nil {
		t.Fatalf("custody: %v", err)
	}
	identities := identity.NewService(db, chain.NewSandbox(), custody)
	sessions := NewService(db, identities, time.Hour, 7*24*time.Hour)
	validator := stubValidator{claims: ExternalClaims{
		Issuer: "https://jahazi.example.test", Subject: "fed-subject-1", Name: "Federated Manager",
	}}
	federation := NewFederationService(db, validator, identities, sessions)

	result, err := federation.Login(context.Background(), "signed-id-token", "nonce")
	if err != nil {
		t.Fatalf("federated login: %v", err)
	}
	if result.RefreshToken == "" || result.RefreshExpiresAt == nil {
		t.Fatalf("federated login issued no refresh grant: %+v", result)
	}
	renewed, err := sessions.Refresh(result.RefreshToken)
	if err != nil {
		t.Fatalf("federated refresh grant unusable: %v", err)
	}
	if renewed.User.ID != result.User.ID || renewed.SessionToken == "" {
		t.Fatalf("refresh returned the wrong account: %+v", renewed)
	}
}