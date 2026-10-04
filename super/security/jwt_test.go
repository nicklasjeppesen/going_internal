package security

import (
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/constants"
)

func TestTokenCarriesSessionID(t *testing.T) {
	t.Setenv(constants.APP_Key, "test-key-0123456789abcdef0123456")
	svc := NewJWTService()

	token, err := svc.GenerateForSession(42, "session-abc")
	if err != nil {
		t.Fatal(err)
	}
	_, claims, err := svc.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "42" || claims.SessionID != "session-abc" {
		t.Fatalf("claims = %q / %q, want 42 / session-abc", claims.Subject, claims.SessionID)
	}

	// A token signed with another key is rejected
	other := &JWTService{secret: []byte("another-key")}
	if _, _, err := other.Verify(token); err == nil {
		t.Fatal("token verified with the wrong key")
	}
}
