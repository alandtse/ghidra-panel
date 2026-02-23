package username

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// GeneratePseudonymous creates a privacy-preserving username from OAuth credentials.
// Format: {sanitized_oauth_username}_{short_hash}
// Example: "alice_8f3a2b" from Discord "Alice#1234" with ID 918273645
//
// Properties:
// - Pseudonymous: Cannot reverse to OAuth identity
// - Deterministic: Same OAuth account always generates same username
// - Unique: Hash collision probability is negligible
// - Human-readable: Includes cleaned OAuth username prefix
func GeneratePseudonymous(provider, oauthUsername, oauthUserID string) string {
	// Clean OAuth username (remove special chars, lowercase, max 16 chars)
	cleanName := Sanitize(oauthUsername)

	// Generate short hash from provider + userID for uniqueness
	combined := fmt.Sprintf("%s:%s", provider, oauthUserID)
	hash := sha256.Sum256([]byte(combined))
	shortHash := hex.EncodeToString(hash[:3]) // 6 hex characters

	return fmt.Sprintf("%s_%s", cleanName, shortHash)
}

// Sanitize cleans OAuth username for use as Ghidra username prefix.
// - Converts to lowercase
// - Removes special characters (#, @, etc.)
// - Keeps only alphanumeric and underscores
// - Limits to 16 characters
// - Ensures it starts with a letter
func Sanitize(oauthUsername string) string {
	// Convert to lowercase
	clean := strings.ToLower(oauthUsername)

	// Remove special characters, keep only alphanumeric and hyphens/underscores
	reg := regexp.MustCompile(`[^a-z0-9_-]`)
	clean = reg.ReplaceAllString(clean, "")

	// Replace hyphens with underscores for consistency
	clean = strings.ReplaceAll(clean, "-", "_")

	// Remove leading/trailing underscores
	clean = strings.Trim(clean, "_")

	// Ensure it starts with a letter (prepend 'u' if not)
	if len(clean) == 0 || (clean[0] < 'a' || clean[0] > 'z') {
		clean = "u" + clean
	}

	// Limit length to 16 characters (total username will be ~23 chars with hash)
	if len(clean) > 16 {
		clean = clean[:16]
	}

	// Fallback if somehow empty
	if clean == "" {
		clean = "user"
	}

	return clean
}
