package web

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.mkw.re/ghidra-panel/database"
)

// setupAdminTestDB creates a test database with seeded data for admin panel testing
func setupAdminTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "admin_test.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Seed test data programmatically
	if err := seedAdminTestData(db); err != nil {
		t.Fatalf("failed to seed test data: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

// seedAdminTestData populates the database with test data
func seedAdminTestData(db *database.DB) error {
	ctx := context.Background()

	// Create super admin
	if err := db.CreateAccountAsSuperAdmin(ctx, 100001, "admin_super", "admin123", "discord"); err != nil {
		return err
	}

	// Create regular users from different providers
	users := []struct {
		id       uint64
		username string
		password string
		provider string
	}{
		{200001, "alice_d1a2b3", "alice123", "discord"},
		{200002, "bob_c4d5e6", "bob123", "discord"},
		{200003, "charlie_f7g8h9", "charlie123", "discord"},
		{300001, "diana_i1j2k3", "diana123", "github"},
		{300002, "eve_l4m5n6", "eve123", "github"},
		{400001, "frank_o7p8q9", "frank123", "google"},
		{500001, "grace_r1s2t3", "grace123", "gitlab"},
	}

	for _, u := range users {
		if err := db.CreateAccount(ctx, u.id, u.username, u.password, u.provider); err != nil {
			return err
		}
	}

	// Create audit log entries
	if err := seedAuditLogs(db, ctx); err != nil {
		return err
	}

	return nil
}

func seedAuditLogs(db *database.DB, ctx context.Context) error {
	now := time.Now()

	// Successful logins
	logins := []struct {
		offset   time.Duration
		userID   uint64
		username string
		ip       string
	}{
		{30 * time.Minute, 100001, "admin_super", "192.168.1.100"},
		{1 * time.Hour, 200003, "charlie_f7g8h9", "10.0.0.50"},
		{6 * time.Hour, 400001, "frank_o7p8q9", "203.0.113.45"},
	}

	for _, login := range logins {
		userID := login.userID
		if err := db.CreateAuditLog(ctx, &database.AuditLogEntry{
			Timestamp:    now.Add(-login.offset).UnixMilli(),
			UserID:       &userID,
			Username:     login.username,
			Action:       database.ActionLogin,
			ResourceType: database.ResourceTypeSession,
			IPAddress:    login.ip,
			UserAgent:    "Mozilla/5.0",
			Success:      true,
		}); err != nil {
			return err
		}
	}

	// Failed login attempts
	failedLogins := []struct {
		offset   time.Duration
		username string
		ip       string
	}{
		{45 * time.Minute, "unknown_user", "198.51.100.78"},
		{3 * time.Hour, "attacker123", "185.220.101.45"},
		{18 * time.Hour, "hacker", "45.142.212.61"},
	}

	for _, failed := range failedLogins {
		if err := db.CreateAuditLogWithDetails(ctx, nil, failed.username, database.ActionLoginFailed,
			database.ResourceTypeSession, "", failed.ip, "python-requests/2.31.0", false,
			map[string]interface{}{"reason": "invalid_credentials"}); err != nil {
			return err
		}
	}

	return nil
}

func TestAdminPanel_Authentication(t *testing.T) {
	db, cleanup := setupAdminTestDB(t)
	defer cleanup()

	tests := []struct {
		name      string
		userID    uint64
		provider  string
		wantAdmin bool
	}{
		{
			name:      "super admin can access",
			userID:    100001,
			provider:  "discord",
			wantAdmin: true,
		},
		{
			name:      "regular user cannot access",
			userID:    200001,
			provider:  "discord",
			wantAdmin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if user is super admin
			isAdmin, err := db.IsSuperAdmin(context.Background(), tt.userID, tt.provider)
			if err != nil {
				t.Fatalf("IsSuperAdmin() error = %v", err)
			}

			if isAdmin != tt.wantAdmin {
				t.Errorf("IsSuperAdmin() = %v, want %v", isAdmin, tt.wantAdmin)
			}
		})
	}
}

func TestAdminPanel_UserListing(t *testing.T) {
	db, cleanup := setupAdminTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Count total users
	count, err := db.CountAccounts(ctx)
	if err != nil {
		t.Fatalf("CountAccounts() error = %v", err)
	}

	// Should have 8 users (1 admin + 7 regular users from seed data)
	expectedUsers := 8
	if count != expectedUsers {
		t.Errorf("expected %d users, got %d", expectedUsers, count)
	}

	// Verify users exist
	expectedUsernames := []string{
		"admin_super",
		"alice_d1a2b3",
		"bob_c4d5e6",
		"charlie_f7g8h9",
		"diana_i1j2k3",
		"eve_l4m5n6",
		"frank_o7p8q9",
		"grace_r1s2t3",
	}

	for _, username := range expectedUsernames {
		exists, err := db.UsernameExists(ctx, username)
		if err != nil {
			t.Errorf("UsernameExists(%s) error = %v", username, err)
		}
		if !exists {
			t.Errorf("expected user %s to exist", username)
		}
	}
}

func TestAdminPanel_AuditLogRetrieval(t *testing.T) {
	db, cleanup := setupAdminTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Get recent audit logs
	logs, err := db.GetAuditLogs(ctx, 100, 0)
	if err != nil {
		t.Fatalf("GetAuditLogs() error = %v", err)
	}

	// Should have logs from seed data (3 logins + 3 failed logins = 6 minimum)
	minExpectedLogs := 6
	if len(logs) < minExpectedLogs {
		t.Errorf("expected at least %d audit logs, got %d", minExpectedLogs, len(logs))
	}

	// Verify logs have required fields
	for i, log := range logs {
		if log.Action == "" {
			t.Errorf("log %d has empty action", i)
		}
		if log.Timestamp == 0 {
			t.Errorf("log %d has zero timestamp", i)
		}
		// Username can be empty for failed logins
		if log.Success && log.Username == "" {
			t.Errorf("log %d has empty username but success=true", i)
		}
	}
}

func TestAdminPanel_FailedLoginTracking(t *testing.T) {
	db, cleanup := setupAdminTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Count failed logins in the last 24 hours
	since := time.Now().Add(-24 * time.Hour)
	count, err := db.GetFailedLoginCount(ctx, since)
	if err != nil {
		t.Fatalf("GetFailedLoginCount() error = %v", err)
	}

	// Seed data includes 3 failed login attempts in the last 24 hours
	expectedFailures := 3
	if count != expectedFailures {
		t.Errorf("expected %d failed logins, got %d", expectedFailures, count)
	}
}

func TestAdminPanel_ProviderDistribution(t *testing.T) {
	db, cleanup := setupAdminTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Count users by provider
	providerCounts := make(map[string]int)

	// Query to count by provider
	rows, err := db.QueryContext(ctx, "SELECT provider, COUNT(*) FROM passwords GROUP BY provider")
	if err != nil {
		t.Fatalf("failed to query provider counts: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var provider string
		var count int
		if err := rows.Scan(&provider, &count); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		providerCounts[provider] = count
	}

	// Verify expected distribution from seed data
	expectedCounts := map[string]int{
		"discord": 4, // admin_super + alice + bob + charlie
		"github":  2, // diana + eve
		"google":  1, // frank
		"gitlab":  1, // grace
	}

	for provider, expectedCount := range expectedCounts {
		actualCount := providerCounts[provider]
		if actualCount != expectedCount {
			t.Errorf("provider %s: expected %d users, got %d", provider, expectedCount, actualCount)
		}
	}
}

func TestAdminPanel_DataIntegrity(t *testing.T) {
	db, cleanup := setupAdminTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test 1: All users should be retrievable
	count, err := db.CountAccounts(ctx)
	if err != nil {
		t.Fatalf("CountAccounts() error = %v", err)
	}
	if count == 0 {
		t.Error("expected users to be seeded")
	}

	// Test 2: Audit logs should be ordered by timestamp (descending)
	logs, err := db.GetAuditLogs(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetAuditLogs() error = %v", err)
	}
	for i := 1; i < len(logs); i++ {
		if logs[i].Timestamp > logs[i-1].Timestamp {
			t.Errorf("audit logs not ordered by timestamp (desc): log[%d].Timestamp=%d > log[%d].Timestamp=%d",
				i, logs[i].Timestamp, i-1, logs[i-1].Timestamp)
		}
	}

	// Test 3: Failed logins should have success=false
	failedLogins, err := db.GetRecentFailedLogins(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentFailedLogins() error = %v", err)
	}
	for i, log := range failedLogins {
		if log.Success {
			t.Errorf("failed login %d has success=true", i)
		}
		if log.Action != database.ActionLoginFailed {
			t.Errorf("failed login %d has action=%s, want %s", i, log.Action, database.ActionLoginFailed)
		}
	}

	t.Log("Admin panel data integrity checks passed")
}

// BenchmarkAdminPanel_AuditLogQuery benchmarks audit log retrieval with realistic data
func BenchmarkAdminPanel_AuditLogQuery(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	db, err := database.Open(dbPath)
	if err != nil {
		b.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Seed test data
	if err := seedAdminTestData(db); err != nil {
		b.Fatalf("failed to seed test data: %v", err)
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := db.GetAuditLogs(ctx, 50, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}
