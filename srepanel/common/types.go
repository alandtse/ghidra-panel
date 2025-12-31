package common

import "go.mkw.re/ghidra-panel/ghidra"

type Identity struct {
	ID         uint64 `json:"id"`
	Username   string `json:"username"`
	AvatarHash string `json:"avatar"`
	Provider   string `json:"provider"` // OAuth provider: "discord", "github", "google", "gitlab", etc.
}

type GhidraEndpoint struct {
	Hostname string `yaml:"host"` // Note: config uses "host", not "hostname"
	Port     uint16 `yaml:"port"`
}

type UserState struct {
	Username    string
	HasPassword bool
}

type Link struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type UserRepoAccessDisplay struct {
	Repo    string
	Perm    ghidra.Permission
	IsAdmin bool
	Stats   *RepositoryStats
}

type RepoUserAccessDisplay struct {
	User string
	Perm ghidra.Permission
}

type Repository struct {
	Name       string
	WebhookURL string
	Stats      *RepositoryStats
}

type RepositoryStats struct {
	SizeBytes        int64
	FileCount        int32
	UserCount        int32
	CreatedTime      int64
	LastModifiedTime int64
}
