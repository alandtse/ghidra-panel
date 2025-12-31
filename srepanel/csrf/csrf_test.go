package csrf

import (
	"testing"
)

func TestOneTime_IssueAndCheck(t *testing.T) {
	ot := NewOneTime()

	// Issue a token
	token := ot.Issue()
	if token == "" {
		t.Fatal("Issue() returned empty token")
	}

	// Check should succeed
	id, err := ot.Check(token)
	if err != nil {
		t.Errorf("Check() error = %v", err)
	}
	if id == 0 {
		t.Error("Check() returned zero ID")
	}

	// Check again should still succeed (not consumed yet)
	id2, err := ot.Check(token)
	if err != nil {
		t.Errorf("second Check() error = %v", err)
	}
	if id != id2 {
		t.Errorf("Check() returned different IDs: %q vs %q", id, id2)
	}
}

func TestOneTime_ConsumeInvalidatesToken(t *testing.T) {
	ot := NewOneTime()

	token := ot.Issue()
	id, err := ot.Check(token)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	// Consume the token
	err = ot.Consume(id)
	if err != nil {
		t.Errorf("Consume() error = %v", err)
	}

	// Trying to consume again should fail (double consumption)
	err = ot.Consume(id)
	if err == nil {
		t.Error("expected error when consuming token twice, got nil")
	}
}

func TestOneTime_ConsumeNonExistent(t *testing.T) {
	ot := NewOneTime()

	// Try to consume an ID that was never issued (outside the ring buffer)
	err := ot.Consume(99999999)
	if err == nil {
		t.Error("expected error when consuming non-existent token, got nil")
	}
}

func TestOneTime_CheckInvalidToken(t *testing.T) {
	ot := NewOneTime()

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"invalid token", "invalid-token"},
		{"random string", "abcdef123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ot.Check(tt.token)
			if err == nil {
				t.Errorf("expected error for %q, got nil", tt.token)
			}
		})
	}
}

func TestOneTime_MultipleTokens(t *testing.T) {
	ot := NewOneTime()

	// Issue multiple tokens
	tokens := make([]string, 10)
	for i := range tokens {
		tokens[i] = ot.Issue()
	}

	// All tokens should be unique
	seen := make(map[string]bool)
	for _, token := range tokens {
		if seen[token] {
			t.Errorf("duplicate token issued: %q", token)
		}
		seen[token] = true
	}

	// All tokens should be valid
	for i, token := range tokens {
		_, err := ot.Check(token)
		if err != nil {
			t.Errorf("token %d check failed: %v", i, err)
		}
	}
}

func TestOneTime_TokenFormat(t *testing.T) {
	ot := NewOneTime()

	token := ot.Issue()

	// Token should be non-empty
	if token == "" {
		t.Fatal("Issue() returned empty token")
	}

	// Token should start with "v0:" prefix
	if len(token) < 3 || token[:3] != "v0:" {
		t.Errorf("expected token to start with 'v0:', got %q", token)
	}

	// Token should be a reasonable length (v0: + base64 encoded)
	if len(token) < 10 {
		t.Errorf("token too short: %d characters", len(token))
	}

	// After "v0:", should only contain base64 URL-safe characters
	tokenData := token[3:]
	for _, c := range tokenData {
		valid := (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '='
		if !valid {
			t.Errorf("token data contains invalid character: %q in %q", c, tokenData)
		}
	}
}

func TestOneTime_ConcurrentAccess(t *testing.T) {
	ot := NewOneTime()

	// Issue tokens concurrently
	done := make(chan string, 100)
	for i := 0; i < 100; i++ {
		go func() {
			done <- ot.Issue()
		}()
	}

	// Collect all tokens
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := <-done
		if tokens[token] {
			t.Errorf("duplicate token in concurrent issuance: %q", token)
		}
		tokens[token] = true
	}

	// Verify all tokens concurrently
	errors := make(chan error, len(tokens))
	for token := range tokens {
		token := token
		go func() {
			_, err := ot.Check(token)
			errors <- err
		}()
	}

	for i := 0; i < len(tokens); i++ {
		if err := <-errors; err != nil {
			t.Errorf("concurrent Check() error: %v", err)
		}
	}
}

func TestOneTime_DoubleConsume(t *testing.T) {
	// This test verifies that consuming the same token twice fails
	ot := NewOneTime()

	// Issue and consume token
	token := ot.Issue()
	id, err := ot.Check(token)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	// First consumption should succeed
	err = ot.Consume(id)
	if err != nil {
		t.Fatalf("first Consume() error = %v", err)
	}

	// Second consumption should fail
	err = ot.Consume(id)
	if err == nil {
		t.Error("expected error on double consumption, got nil")
	}
	
	// Verify error message indicates reuse
	if err != nil && err.Error() != "csrf reuse detected" {
		t.Errorf("expected 'csrf reuse detected' error, got: %v", err)
	}
}

func BenchmarkOneTime_Issue(b *testing.B) {
	ot := NewOneTime()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = ot.Issue()
	}
}

func BenchmarkOneTime_Check(b *testing.B) {
	ot := NewOneTime()
	token := ot.Issue()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ot.Check(token)
	}
}

func BenchmarkOneTime_IssueAndCheck(b *testing.B) {
	ot := NewOneTime()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		token := ot.Issue()
		_, _ = ot.Check(token)
	}
}

func BenchmarkOneTime_FullCycle(b *testing.B) {
	ot := NewOneTime()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		token := ot.Issue()
		id, err := ot.Check(token)
		if err != nil {
			b.Fatal(err)
		}
		_ = ot.Consume(id)
	}
}
