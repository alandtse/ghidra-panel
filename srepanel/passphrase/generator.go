package passphrase

import (
	"strings"

	"github.com/sethvargo/go-diceware/diceware"
)

// Generate creates a memorable passphrase using the Diceware algorithm.
// It generates 4 words from the EFF Short Wordlist (1,296 words).
//
// Security: 4 words = ~41 bits of entropy (1,296^4 combinations)
// This is sufficient for collaborative RE projects while being memorable.
//
// Example output: "alpine-rocket-marble-sunset"
func Generate() (string, error) {
	// Generate 4 random words using EFF Short Wordlist
	words, err := diceware.Generate(4)
	if err != nil {
		return "", err
	}

	// Join with hyphens for easy typing
	return strings.Join(words, "-"), nil
}

// MustGenerate is like Generate but panics on error.
// Use this in contexts where error handling is not critical.
func MustGenerate() string {
	passphrase, err := Generate()
	if err != nil {
		panic(err)
	}
	return passphrase
}
