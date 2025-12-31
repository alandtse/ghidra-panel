package common

// ProviderMetadata contains display information for OAuth providers
type ProviderMetadata struct {
	Name        string // Internal name (e.g., "discord", "github", "google")
	DisplayName string // Human-readable name (e.g., "Discord", "GitHub", "Google")
	IconURL     string // Optional URL to provider icon/logo
}
