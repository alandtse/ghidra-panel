package token

import (
	"testing"
	"time"

	"go.mkw.re/ghidra-panel/common"
)

func TestIssueTokenValidity(t *testing.T) {
	secret := []byte("secret")

	tests := []struct {
		name     string
		validity time.Duration
	}{
		{"ShortValidity", 1 * time.Second},
		{"LongValidity", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail initially because NewIssuer doesn't accept validity yet
			// and Issue uses a hardcoded constant.
			issuer := NewIssuer(secret, tt.validity)

			ident := &common.Identity{
				ID:       1,
				Username: "testuser",
			}

			_, exp := issuer.Issue(ident)

			// Check if expiration is close to expected time (allowing for small delta)
			expected := time.Now().Add(tt.validity)
			diff := expected.Sub(exp)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("expected expiration around %v, got %v (diff: %v)", expected, exp, diff)
			}
		})
	}
}

func TestVerifyToken(t *testing.T) {
	secret := []byte("secret")
	validity := 1 * time.Hour
	issuer := NewIssuer(secret, validity)

	ident := &common.Identity{
		ID:         123,
		Username:   "alice",
		AvatarHash: "hash123",
		Provider:   "github",
	}

	tokenString, _ := issuer.Issue(ident)

	got, err := issuer.Verify(tokenString)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if got.ID != ident.ID {
		t.Errorf("got ID %d, want %d", got.ID, ident.ID)
	}
	if got.Username != ident.Username {
		t.Errorf("got Username %q, want %q", got.Username, ident.Username)
	}
}
