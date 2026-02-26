package main

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
	"go.mkw.re/ghidra-panel/common"
)

// OAuthProviderConfig represents configuration for an OAuth provider
type OAuthProviderConfig struct {
	Enabled      bool   `yaml:"enabled" env:"ENABLED"`
	ClientID     string `yaml:"client_id" env:"CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" env:"CLIENT_SECRET"`
	// Type specifies the provider type: "oauth2" or "oidc"
	// Default is "oidc" if issuer_url is set, "oauth2" otherwise
	Type string `yaml:"type,omitempty" env:"TYPE"`

	// OIDC-specific configuration
	// IssuerURL is used for OIDC providers (e.g., Google, GitLab)
	IssuerURL string `yaml:"issuer_url,omitempty" env:"ISSUER_URL"`

	// OAuth2-specific configuration
	// AuthURL is the authorization endpoint (e.g., "https://github.com/login/oauth/authorize")
	AuthURL string `yaml:"auth_url,omitempty" env:"AUTH_URL"`
	// TokenURL is the token endpoint (e.g., "https://github.com/login/oauth/access_token")
	TokenURL string `yaml:"token_url,omitempty" env:"TOKEN_URL"`
	// UserInfoURL is the user info endpoint (e.g., "https://api.github.com/user")
	UserInfoURL string `yaml:"user_info_url,omitempty" env:"USER_INFO_URL"`
	// Scopes are the OAuth scopes to request (e.g., ["read:user"])
	Scopes []string `yaml:"scopes,omitempty" env:"SCOPES"`
	// AuthStyle is "params" or "header" (default: "header")
	AuthStyle string `yaml:"auth_style,omitempty" env:"AUTH_STYLE"`

	// Field mapping for user info extraction
	// UserIDField is the JSON field for user ID (default: "id")
	UserIDField string `yaml:"user_id_field,omitempty" env:"USER_ID_FIELD"`
	// UsernameField is the JSON field for username (default: "username")
	UsernameField string `yaml:"username_field,omitempty" env:"USERNAME_FIELD"`
	// AvatarField is the JSON field for avatar URL (optional)
	AvatarField string `yaml:"avatar_field,omitempty" env:"AVATAR_FIELD"`
	// UserIDIsString indicates if user ID is a string that needs parsing (e.g., Discord)
	UserIDIsString bool `yaml:"user_id_is_string,omitempty" env:"USER_ID_IS_STRING"`

	// Display configuration
	// DisplayName is the human-readable name shown on the login button
	// If not set, the provider key will be capitalized (e.g., "okta" → "Okta")
	DisplayName string `yaml:"display_name,omitempty" env:"DISPLAY_NAME"`
	// IconURL is a URL to an icon/logo for the provider
	// If not set, built-in icons will be used for known providers (discord, github, google, gitlab)
	IconURL string `yaml:"icon_url,omitempty" env:"ICON_URL"`
}

type config struct {
	ConfigVersion int    `yaml:"config_version,omitempty" env:"SRE_CONFIG_VERSION"` // Optional: helps track config format changes
	CommunityName string `yaml:"community_name,omitempty" env:"SRE_COMMUNITY_NAME"` // Optional: display name for your community/server
	BaseURL       string `yaml:"base_url" env:"SRE_BASE_URL"`
	// Legacy Discord config (deprecated, but kept for backwards compatibility)
	Discord struct {
		BotToken     string `yaml:"bot_token" env:"SRE_DISCORD_BOT_TOKEN"`
		ClientID     string `yaml:"client_id" env:"SRE_DISCORD_CLIENT_ID"`
		ClientSecret string `yaml:"client_secret" env:"SRE_DISCORD_CLIENT_SECRET"`
		WebhookURL   string `yaml:"webhook_url" env:"SRE_DISCORD_WEBHOOK_URL"`
	} `yaml:"discord"`
	// OAuth providers config - supports both fixed providers and dynamic OIDC providers
	// You can add any OIDC-compliant provider by adding a new key
	OAuth  map[string]OAuthProviderConfig `yaml:"oauth" env-prefix:"SRE_OAUTH_"`
	Ghidra struct {
		Endpoint common.GhidraEndpoint `yaml:"endpoint"`
		GRPCAddr string                `yaml:"grpc_addr" env:"SRE_GHIDRA_GRPC_ADDR"`
	} `yaml:"ghidra"`
	Links             []common.Link `yaml:"links"`
	SuperAdmins       []string      `yaml:"super_admins" env:"SRE_SUPER_ADMINS"`
	FirstUserIsAdmin  bool          `yaml:"first_user_is_admin" env:"SRE_FIRST_USER_IS_ADMIN"`
	GeoIPDatabase     string        `yaml:"geoip_database" env:"SRE_GEOIP_DATABASE"`
	MaxMindAccountID  string        `yaml:"maxmind_account_id" env:"SRE_MAXMIND_ACCOUNT_ID"`
	MaxMindLicenseKey string        `yaml:"maxmind_license_key" env:"SRE_MAXMIND_LICENSE_KEY"`

	// Optional: Number of days to retain audit logs (default: 90)
	// Old logs are automatically cleaned up daily
	AuditLogRetentionDays int `yaml:"audit_log_retention_days" env:"SRE_AUDIT_LOG_RETENTION_DAYS"`

	// Optional: Number of days token is valid (default: 90)
	TokenValidityDays int `yaml:"token_validity_days,omitempty" env:"SRE_TOKEN_VALIDITY_DAYS"`
}

func (c *config) validate() {
	// Warn if using old config format (no version or old version)
	const currentConfigVersion = 1
	if c.ConfigVersion == 0 {
		log.Println("WARNING: Your config file doesn't have a 'config_version' field.")
		log.Println("         See CONFIG_CHANGELOG.md for migration guide.")
	} else if c.ConfigVersion < currentConfigVersion {
		log.Printf("WARNING: Your config version is %d, but current version is %d.\n", c.ConfigVersion, currentConfigVersion)
		log.Println("         See CONFIG_CHANGELOG.md for what's new and how to update.")
	}

	if c.BaseURL == "" {
		log.Fatal("base_url not set")
	}

	// Check if at least one OAuth provider is configured
	hasProvider := false

	// Support legacy Discord config
	if c.Discord.ClientID != "" && c.Discord.ClientSecret != "" {
		hasProvider = true
	}

	// Check OAuth providers
	for name, provider := range c.OAuth {
		if !provider.Enabled {
			continue
		}

		// Auto-disable providers with placeholder credentials
		if isPlaceholder(provider.ClientID) || isPlaceholder(provider.ClientSecret) {
			log.Printf("oauth.%s: auto-disabled (contains placeholder credentials)", name)
			provider.Enabled = false
			c.OAuth[name] = provider
			continue
		}

		if provider.ClientID == "" || provider.ClientSecret == "" {
			log.Fatalf("oauth.%s: client_id and client_secret must be set", name)
		}

		// Determine provider type
		providerType := provider.Type
		if providerType == "" {
			if provider.IssuerURL != "" {
				providerType = "oidc"
			} else if provider.AuthURL != "" && provider.TokenURL != "" {
				providerType = "oauth2"
			} else {
				log.Fatalf("oauth.%s: must specify either issuer_url (OIDC) or auth_url+token_url (OAuth2)", name)
			}
		}

		// Validate based on type
		if providerType == "oidc" {
			if provider.IssuerURL == "" {
				log.Fatalf("oauth.%s: issuer_url must be set for OIDC providers", name)
			}
		} else if providerType == "oauth2" {
			if provider.AuthURL == "" {
				log.Fatalf("oauth.%s: auth_url must be set for OAuth2 providers", name)
			}
			if provider.TokenURL == "" {
				log.Fatalf("oauth.%s: token_url must be set for OAuth2 providers", name)
			}
			if provider.UserInfoURL == "" {
				log.Fatalf("oauth.%s: user_info_url must be set for OAuth2 providers", name)
			}
		} else {
			log.Fatalf("oauth.%s: unknown provider type '%s' (must be 'oauth2' or 'oidc')", name, providerType)
		}

		hasProvider = true
	}

	if !hasProvider {
		log.Fatal("at least one OAuth provider must be configured")
	}
}

// isPlaceholder checks if a config value is a placeholder that needs to be replaced
func isPlaceholder(value string) bool {
	value = strings.ToLower(value)
	placeholders := []string{
		"xxx",
		"your_",
		"placeholder",
		"change_me",
		"replace_me",
		"example",
	}

	for _, placeholder := range placeholders {
		if strings.Contains(value, placeholder) {
			return true
		}
	}

	return false
}

// loadConfig loads configuration from either JSON or YAML file
// It also reads from the environment variables utilizing cleanenv.
func loadConfig(path string) (config, error) {
	var cfg config

	err := cleanenv.ReadConfig(path, &cfg)
	if err != nil {
		// If the file doesn't exist, we still want to read env vars
		if errors.Is(err, os.ErrNotExist) {
			err = cleanenv.ReadEnv(&cfg)
			if err != nil {
				return cfg, err
			}
			return cfg, nil
		}
		return cfg, err
	}

	return cfg, nil
}
