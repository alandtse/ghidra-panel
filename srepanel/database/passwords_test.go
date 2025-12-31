package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.mkw.re/ghidra-panel/common"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Migrations run automatically in Open()

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestCreateAccount(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name     string
		id       uint64
		username string
		password string
		provider string
		wantErr  bool
	}{
		{
			name:     "valid account creation",
			id:       12345,
			username: "testuser",
			password: "testpass123",
			provider: "discord",
			wantErr:  false,
		},
		{
			name:     "duplicate account should fail",
			id:       12345,
			username: "testuser2",
			password: "testpass456",
			provider: "discord",
			wantErr:  true,
		},
		{
			name:     "different ID same provider should succeed",
			id:       67890,
			username: "testuser_different",
			password: "testpass789",
			provider: "discord",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.CreateAccount(ctx, tt.id, tt.username, tt.password, tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateAccount() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify account was created
				state, err := db.GetUserState(ctx, &common.Identity{
					ID:       tt.id,
					Provider: tt.provider,
				})
				if err != nil {
					t.Errorf("GetUserState() after creation failed: %v", err)
				}
				if state.Username != tt.username {
					t.Errorf("expected username %q, got %q", tt.username, state.Username)
				}
				if !state.HasPassword {
					t.Error("expected HasPassword to be true")
				}
			}
		})
	}
}

func TestGetUserState(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create test account
	id := uint64(99999)
	username := "testuser"
	password := "testpass"
	provider := "discord"

	err := db.CreateAccount(ctx, id, username, password, provider)
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	tests := []struct {
		name         string
		identity     *common.Identity
		wantUsername string
		wantHasPass  bool
		wantErr      bool
	}{
		{
			name: "existing account",
			identity: &common.Identity{
				ID:       id,
				Provider: provider,
			},
			wantUsername: username,
			wantHasPass:  true,
			wantErr:      false,
		},
		{
			name: "non-existent account",
			identity: &common.Identity{
				ID:       88888,
				Provider: provider,
			},
			wantHasPass: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := db.GetUserState(ctx, tt.identity)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if state.HasPassword != tt.wantHasPass {
					t.Errorf("expected HasPassword=%v, got %v", tt.wantHasPass, state.HasPassword)
				}
				if tt.wantHasPass && state.Username != tt.wantUsername {
					t.Errorf("expected username %q, got %q", tt.wantUsername, state.Username)
				}
			}
		})
	}
}

func TestUsernameExists(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create test account
	err := db.CreateAccount(ctx, 11111, "existinguser", "pass123", "discord")
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	tests := []struct {
		name       string
		username   string
		wantExists bool
	}{
		{
			name:       "existing username",
			username:   "existinguser",
			wantExists: true,
		},
		{
			name:       "non-existent username",
			username:   "nonexistent",
			wantExists: false,
		},
		{
			name:       "case sensitivity",
			username:   "ExistingUser",
			wantExists: false, // SQLite is case-sensitive by default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := db.UsernameExists(ctx, tt.username)
			if err != nil {
				t.Errorf("UsernameExists() error = %v", err)
				return
			}
			if exists != tt.wantExists {
				t.Errorf("UsernameExists(%q) = %v, want %v", tt.username, exists, tt.wantExists)
			}
		})
	}
}

func TestUpdatePassword(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create test account
	id := uint64(22222)
	username := "testuser"
	oldPassword := "oldpass123"
	newPassword := "newpass456"
	provider := "discord"

	err := db.CreateAccount(ctx, id, username, oldPassword, provider)
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	// Update password
	err = db.UpdatePassword(ctx, id, newPassword, provider)
	if err != nil {
		t.Errorf("UpdatePassword() error = %v", err)
	}

	// Verify account still exists with same username
	state, err := db.GetUserState(ctx, &common.Identity{ID: id, Provider: provider})
	if err != nil {
		t.Fatalf("GetUserState() after update failed: %v", err)
	}

	if state.Username != username {
		t.Errorf("username changed after password update: got %q, want %q", state.Username, username)
	}

	if !state.HasPassword {
		t.Error("expected HasPassword to be true after update")
	}
}

func TestUpdatePassword_NonExistentUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	err := db.UpdatePassword(ctx, 99999, "newpass", "discord")
	if err == nil {
		t.Error("expected error when updating password for non-existent user")
	}
}

func TestCreateAccountAsSuperAdmin(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	id := uint64(33333)
	username := "admin"
	password := "adminpass"
	provider := "discord"

	err := db.CreateAccountAsSuperAdmin(ctx, id, username, password, provider)
	if err != nil {
		t.Fatalf("CreateAccountAsSuperAdmin() error = %v", err)
	}

	// Verify super admin flag
	isAdmin, err := db.IsSuperAdmin(ctx, id, provider)
	if err != nil {
		t.Fatalf("IsSuperAdmin() error = %v", err)
	}

	if !isAdmin {
		t.Error("expected user to be super admin")
	}
}

func TestIsSuperAdmin(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create regular user
	regularID := uint64(44444)
	err := db.CreateAccount(ctx, regularID, "regular", "pass123", "discord")
	if err != nil {
		t.Fatalf("failed to create regular account: %v", err)
	}

	// Create super admin
	adminID := uint64(55555)
	err = db.CreateAccountAsSuperAdmin(ctx, adminID, "admin", "pass456", "discord")
	if err != nil {
		t.Fatalf("failed to create admin account: %v", err)
	}

	tests := []struct {
		name     string
		id       uint64
		provider string
		wantErr  bool
		isAdmin  bool
	}{
		{
			name:     "regular user",
			id:       regularID,
			provider: "discord",
			wantErr:  false,
			isAdmin:  false,
		},
		{
			name:     "super admin",
			id:       adminID,
			provider: "discord",
			wantErr:  false,
			isAdmin:  true,
		},
		{
			name:     "non-existent user",
			id:       88888,
			provider: "discord",
			wantErr:  true,
			isAdmin:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isAdmin, err := db.IsSuperAdmin(ctx, tt.id, tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsSuperAdmin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && isAdmin != tt.isAdmin {
				t.Errorf("IsSuperAdmin() = %v, want %v", isAdmin, tt.isAdmin)
			}
		})
	}
}

func TestCountAccounts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Initially should be 0
	count, err := db.CountAccounts(ctx)
	if err != nil {
		t.Fatalf("CountAccounts() error = %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 accounts, got %d", count)
	}

	// Create multiple accounts
	accounts := []struct {
		id       uint64
		username string
		provider string
	}{
		{1, "user1", "discord"},
		{2, "user2", "github"},
		{3, "user3", "discord"},
	}

	for _, acc := range accounts {
		err := db.CreateAccount(ctx, acc.id, acc.username, "pass123", acc.provider)
		if err != nil {
			t.Fatalf("failed to create account: %v", err)
		}
	}

	// Should now have 3 accounts
	count, err = db.CountAccounts(ctx)
	if err != nil {
		t.Fatalf("CountAccounts() error = %v", err)
	}
	if count != len(accounts) {
		t.Errorf("expected %d accounts, got %d", len(accounts), count)
	}
}

func TestHashPassword_Deterministic(t *testing.T) {
	password := "testpass123"

	hash1, salt1, err1 := hashPassword(password)
	hash2, salt2, err2 := hashPassword(password)

	if err1 != nil || err2 != nil {
		t.Fatalf("hashPassword() errors: %v, %v", err1, err2)
	}

	// Salts should be different (random)
	if salt1 == salt2 {
		t.Error("expected different salts for each hash")
	}

	// Hashes should be different (due to different salts)
	if string(hash1) == string(hash2) {
		t.Error("expected different hashes with different salts")
	}

	// Hash length should be 32 bytes (Argon2id output)
	if len(hash1) != 32 {
		t.Errorf("expected hash length 32, got %d", len(hash1))
	}

	// Salt length should be 16 bytes
	if len(salt1) != 16 {
		t.Errorf("expected salt length 16, got %d", len(salt1))
	}
}

func TestSetUsername(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create test account
	id := uint64(66666)
	oldUsername := "oldname"
	newUsername := "newname"
	provider := "discord"

	err := db.CreateAccount(ctx, id, oldUsername, "pass123", provider)
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	// Update username
	err = db.SetUsername(ctx, id, newUsername, provider)
	if err != nil {
		t.Errorf("SetUsername() error = %v", err)
	}

	// Verify username changed
	state, err := db.GetUserState(ctx, &common.Identity{ID: id, Provider: provider})
	if err != nil {
		t.Fatalf("GetUserState() after username change failed: %v", err)
	}

	if state.Username != newUsername {
		t.Errorf("expected username %q, got %q", newUsername, state.Username)
	}
}
