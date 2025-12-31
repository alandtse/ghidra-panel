package username

import (
	"testing"
)

func TestGeneratePseudonymous(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		oauthUsername  string
		oauthUserID    string
		expectedPrefix string
		expectedLen    int
	}{
		{
			name:           "Discord username with discriminator",
			provider:       "discord",
			oauthUsername:  "Alice#1234",
			oauthUserID:    "918273645",
			expectedPrefix: "alice1234_",
			expectedLen:    16, // "alice1234_" + 6 hex chars
		},
		{
			name:           "GitHub username",
			provider:       "github",
			oauthUsername:  "octocat",
			oauthUserID:    "583231",
			expectedPrefix: "octocat_",
			expectedLen:    14,
		},
		{
			name:           "Username with special chars",
			provider:       "discord",
			oauthUsername:  "user@name!123",
			oauthUserID:    "12345",
			expectedPrefix: "username123_",
			expectedLen:    18,
		},
		{
			name:           "Long username gets truncated",
			provider:       "github",
			oauthUsername:  "verylongusernamethatexceedslimit",
			oauthUserID:    "99999",
			expectedPrefix: "verylongusername_", // truncated to 16 + underscore
			expectedLen:    23,
		},
		{
			name:           "Username starting with number",
			provider:       "discord",
			oauthUsername:  "123user",
			oauthUserID:    "555",
			expectedPrefix: "u123user_",
			expectedLen:    15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GeneratePseudonymous(tt.provider, tt.oauthUsername, tt.oauthUserID)

			// Check length
			if len(result) != tt.expectedLen {
				t.Errorf("expected length %d, got %d (result: %s)", tt.expectedLen, len(result), result)
			}

			// Check prefix
			if result[:len(tt.expectedPrefix)] != tt.expectedPrefix {
				t.Errorf("expected prefix %q, got %q", tt.expectedPrefix, result[:len(tt.expectedPrefix)])
			}

			// Check format (prefix_hexhash)
			if result[len(result)-7] != '_' {
				t.Errorf("expected underscore before hash, got %q", result)
			}

			// Verify hash is 6 hex characters
			hash := result[len(result)-6:]
			for _, c := range hash {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("hash contains non-hex character: %q in %q", c, hash)
				}
			}
		})
	}
}

func TestGeneratePseudonymous_Deterministic(t *testing.T) {
	provider := "discord"
	oauthUsername := "testuser"
	oauthUserID := "12345"

	// Generate twice and ensure same result
	result1 := GeneratePseudonymous(provider, oauthUsername, oauthUserID)
	result2 := GeneratePseudonymous(provider, oauthUsername, oauthUserID)

	if result1 != result2 {
		t.Errorf("expected deterministic output, got %q and %q", result1, result2)
	}
}

func TestGeneratePseudonymous_UniquePerProvider(t *testing.T) {
	oauthUsername := "testuser"
	oauthUserID := "12345"

	discordResult := GeneratePseudonymous("discord", oauthUsername, oauthUserID)
	githubResult := GeneratePseudonymous("github", oauthUsername, oauthUserID)

	if discordResult == githubResult {
		t.Errorf("expected different hashes for different providers, got %q", discordResult)
	}

	// Check prefixes are same but hashes differ
	if discordResult[:9] != "testuser_" || githubResult[:9] != "testuser_" {
		t.Errorf("expected same prefix for both")
	}

	if discordResult[9:] == githubResult[9:] {
		t.Errorf("expected different hashes, got same: %q", discordResult[9:])
	}
}

func TestGeneratePseudonymous_UniquePerUserID(t *testing.T) {
	provider := "discord"
	oauthUsername := "testuser"

	result1 := GeneratePseudonymous(provider, oauthUsername, "12345")
	result2 := GeneratePseudonymous(provider, oauthUsername, "67890")

	if result1 == result2 {
		t.Errorf("expected different hashes for different user IDs, got %q", result1)
	}
}

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Alice", "alice"},
		{"Alice#1234", "alice1234"},
		{"user@example.com", "userexamplecom"},
		{"test-user", "test_user"},
		{"Test_User_123", "test_user_123"},
		{"___test___", "test"},
		{"123user", "u123user"},
		{"!@#$%", "u"},
		{"", "u"},
		{"verylongusernamethatexceedslimitandshouldbetrimmed", "verylongusername"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeUsername(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeUsername(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeUsername_AlwaysValid(t *testing.T) {
	// Test various edge cases to ensure output is always valid
	inputs := []string{
		"",
		"123",
		"!@#$%^&*()",
		"___",
		"user",
		"VeryLongUsernameThatShouldBeTruncatedToSixteenCharacters",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			result := sanitizeUsername(input)

			// Must not be empty
			if result == "" {
				t.Errorf("sanitizeUsername(%q) returned empty string", input)
			}

			// Must start with letter
			if result[0] < 'a' || result[0] > 'z' {
				t.Errorf("sanitizeUsername(%q) = %q, must start with letter", input, result)
			}

			// Must not exceed 16 chars
			if len(result) > 16 {
				t.Errorf("sanitizeUsername(%q) = %q, exceeds 16 characters", input, result)
			}

			// Must only contain valid characters
			for _, c := range result {
				if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
					t.Errorf("sanitizeUsername(%q) = %q, contains invalid character %q", input, result, c)
				}
			}
		})
	}
}
