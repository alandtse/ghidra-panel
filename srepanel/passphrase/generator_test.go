package passphrase

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	passphrase, err := Generate()
	if err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}

	// Check format: should be 4 words separated by hyphens
	words := strings.Split(passphrase, "-")
	if len(words) != 4 {
		t.Errorf("expected 4 words, got %d in %q", len(words), passphrase)
	}

	// Each word should be non-empty and contain only lowercase letters
	for i, word := range words {
		if word == "" {
			t.Errorf("word %d is empty in %q", i, passphrase)
		}

		for _, c := range word {
			if c < 'a' || c > 'z' {
				t.Errorf("word %d contains non-lowercase character %q in %q", i, c, passphrase)
			}
		}

		// Words should be reasonable length (EFF short wordlist: 3-9 chars per word)
		if len(word) < 3 || len(word) > 9 {
			t.Errorf("word %d has unexpected length %d: %q", i, len(word), word)
		}
	}

	// Total length should be reasonable (4 words of 3-9 chars + 3 hyphens = 15-39 chars)
	if len(passphrase) < 15 || len(passphrase) > 39 {
		t.Errorf("passphrase length %d out of expected range (15-39): %q", len(passphrase), passphrase)
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	// Generate multiple passphrases and ensure they're different
	// (collision probability is negligible with 1,296^4 combinations)
	seen := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		passphrase, err := Generate()
		if err != nil {
			t.Fatalf("Generate() iteration %d returned error: %v", i, err)
		}

		if seen[passphrase] {
			t.Errorf("duplicate passphrase generated: %q", passphrase)
		}
		seen[passphrase] = true
	}

	if len(seen) != iterations {
		t.Errorf("expected %d unique passphrases, got %d", iterations, len(seen))
	}
}

func TestGenerate_NoSpecialCharacters(t *testing.T) {
	// Ensure passphrases don't contain special characters that could cause issues
	iterations := 50

	for i := 0; i < iterations; i++ {
		passphrase, err := Generate()
		if err != nil {
			t.Fatalf("Generate() iteration %d returned error: %v", i, err)
		}

		// Should only contain lowercase letters and hyphens
		for _, c := range passphrase {
			if !((c >= 'a' && c <= 'z') || c == '-') {
				t.Errorf("passphrase contains unexpected character %q: %q", c, passphrase)
			}
		}

		// Should not start or end with hyphen
		if passphrase[0] == '-' || passphrase[len(passphrase)-1] == '-' {
			t.Errorf("passphrase starts or ends with hyphen: %q", passphrase)
		}

		// Should not have consecutive hyphens
		if strings.Contains(passphrase, "--") {
			t.Errorf("passphrase contains consecutive hyphens: %q", passphrase)
		}
	}
}

func TestMustGenerate(t *testing.T) {
	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustGenerate() panicked: %v", r)
		}
	}()

	passphrase := MustGenerate()

	// Basic validation
	if passphrase == "" {
		t.Error("MustGenerate() returned empty string")
	}

	words := strings.Split(passphrase, "-")
	if len(words) != 4 {
		t.Errorf("MustGenerate() expected 4 words, got %d", len(words))
	}
}

func BenchmarkGenerate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Generate()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMustGenerate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = MustGenerate()
	}
}
