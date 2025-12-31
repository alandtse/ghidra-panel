package oauth

import (
	"net/http"

	"go.mkw.re/ghidra-panel/common"
)

// Provider is the interface that all OAuth providers must implement
type Provider interface {
	// Name returns the unique identifier for this provider (e.g., "discord", "github", "google")
	Name() string

	// AuthURL generates the OAuth authorization URL for the user to visit
	AuthURL() string

	// HandleRedirect processes the OAuth callback and returns the user's identity
	HandleRedirect(wr http.ResponseWriter, req *http.Request) (*common.Identity, error)
}
