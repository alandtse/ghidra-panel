package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_EnvFallback(t *testing.T) {
	// Clear environments
	os.Clearenv()

	// Create a temporary basic config file with just the provider map key defined
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("oauth:\n  github:\n    enabled: true\n"), 0644)

	// Pre-fill environment variables that Cleanenv will read due to our struct tags
	os.Setenv("SRE_BASE_URL", "https://env.example.com")
	os.Setenv("SRE_COMMUNITY_NAME", "Env Community")
	os.Setenv("SRE_OAUTH_GITHUB_CLIENT_ID", "testclient123")
	os.Setenv("SRE_OAUTH_GITHUB_CLIENT_SECRET", "testsecret456")
	os.Setenv("SRE_OAUTH_GITHUB_TYPE", "oauth2")
	os.Setenv("SRE_OAUTH_GITHUB_AUTH_URL", "https://mock.com/auth")
	os.Setenv("SRE_OAUTH_GITHUB_TOKEN_URL", "https://mock.com/token")
	os.Setenv("SRE_OAUTH_GITHUB_USER_INFO_URL", "https://mock.com/user")
	os.Setenv("SRE_OAUTH_GITHUB_ENABLED", "1")

	cfg, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig failed when falling back to env vars: %v", err)
	}

	if cfg.BaseURL != "https://env.example.com" {
		t.Errorf("Expected BaseURL 'https://env.example.com', got '%s'", cfg.BaseURL)
	}

	if cfg.CommunityName != "Env Community" {
		t.Errorf("Expected CommunityName 'Env Community', got '%s'", cfg.CommunityName)
	}
}

func TestIsPlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		expected bool
	}{
		{"Empty string", "", false},
		{"Valid Token", "abcdef123456", false},
		{"Basic Placeholder", "replace_me", true},
		{"Uppercase Placeholder", "YOUR_CLIENT_ID", true},
		{"XXX Placeholder", "client_xxx", true},
		{"Example", "example_secret", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPlaceholder(tt.val); got != tt.expected {
				t.Errorf("isPlaceholder(%q) = %v, expected %v", tt.val, got, tt.expected)
			}
		})
	}
}
