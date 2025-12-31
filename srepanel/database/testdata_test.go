package database

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// SeedTestData populates the database with realistic test data for admin panel testing.
// This is useful for manual testing and integration tests.
func SeedTestData(db *DB) error {
	ctx := context.Background()

	// Create super admin
	if err := createTestSuperAdmin(db, ctx); err != nil {
		return fmt.Errorf("failed to create super admin: %w", err)
	}

	// Create regular users from different providers
	if err := createTestUsers(db, ctx); err != nil {
		return fmt.Errorf("failed to create users: %w", err)
	}

	// Create audit log entries
	if err := createTestAuditLogs(db, ctx); err != nil {
		return fmt.Errorf("failed to create audit logs: %w", err)
	}

	return nil
}

func createTestSuperAdmin(db *DB, ctx context.Context) error {
	return db.CreateAccountAsSuperAdmin(ctx, 100001, "admin_super", "admin123", "discord")
}

func createTestUsers(db *DB, ctx context.Context) error {
	users := []struct {
		id       uint64
		username string
		password string
		provider string
	}{
		// Discord users
		{200001, "alice_d1a2b3", "alice123", "discord"},
		{200002, "bob_c4d5e6", "bob123", "discord"},
		{200003, "charlie_f7g8h9", "charlie123", "discord"},

		// GitHub users
		{300001, "diana_i1j2k3", "diana123", "github"},
		{300002, "eve_l4m5n6", "eve123", "github"},

		// Google users
		{400001, "frank_o7p8q9", "frank123", "google"},

		// GitLab users
		{500001, "grace_r1s2t3", "grace123", "gitlab"},
	}

	for _, u := range users {
		if err := db.CreateAccount(ctx, u.id, u.username, u.password, u.provider); err != nil {
			return fmt.Errorf("failed to create user %s: %w", u.username, err)
		}
	}

	return nil
}

func createTestAuditLogs(db *DB, ctx context.Context) error {
	now := time.Now()

	// Recent successful logins
	logins := []struct {
		offset    time.Duration
		userID    uint64
		username  string
		ip        string
		userAgent string
	}{
		{30 * time.Minute, 100001, "admin_super", "192.168.1.100", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0"},
		{1 * time.Hour, 200003, "charlie_f7g8h9", "10.0.0.50", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.0"},
		{2 * time.Hour, 300002, "eve_l4m5n6", "172.16.0.25", "Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0"},
		{6 * time.Hour, 400001, "frank_o7p8q9", "203.0.113.45", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)"},
		{23 * time.Hour, 200001, "alice_d1a2b3", "192.168.1.50", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0"},
		{22 * time.Hour, 200002, "bob_c4d5e6", "10.0.0.100", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.0"},
	}

	for _, login := range logins {
		timestamp := now.Add(-login.offset).UnixMilli()
		_, err := db.ExecContext(ctx, `
			INSERT INTO audit_logs (timestamp, user_id, username, action, resource_type, ip_address, user_agent, success)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			timestamp, login.userID, login.username, ActionLogin, ResourceTypeSession, login.ip, login.userAgent, true)
		if err != nil {
			return fmt.Errorf("failed to insert login: %w", err)
		}
	}

	// Failed login attempts (simulate attacks)
	failedLogins := []struct {
		offset   time.Duration
		username string
		ip       string
	}{
		{45 * time.Minute, "unknown_user", "198.51.100.78"},
		{3 * time.Hour, "attacker123", "185.220.101.45"},
		{12 * time.Hour, "admin", "185.220.101.46"},
		{18 * time.Hour, "hacker", "45.142.212.61"},
		{5 * time.Hour, "script_kiddie", "91.92.251.103"},
	}

	for _, failed := range failedLogins {
		timestamp := now.Add(-failed.offset).UnixMilli()
		_, err := db.ExecContext(ctx, `
			INSERT INTO audit_logs (timestamp, username, action, resource_type, details, ip_address, user_agent, success)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			timestamp, failed.username, ActionLoginFailed, ResourceTypeSession,
			`{"reason": "invalid_credentials"}`, failed.ip, "python-requests/2.31.0", false)
		if err != nil {
			return fmt.Errorf("failed to insert failed login: %w", err)
		}
	}

	// Account creations
	accountCreations := []struct {
		daysAgo  int
		userID   uint64
		username string
		provider string
		ip       string
	}{
		{30, 200001, "alice_d1a2b3", "discord", "192.168.1.50"},
		{25, 200002, "bob_c4d5e6", "discord", "10.0.0.100"},
		{15, 300001, "diana_i1j2k3", "github", "172.16.0.10"},
		{5, 400001, "frank_o7p8q9", "google", "203.0.113.45"},
	}

	for _, acc := range accountCreations {
		timestamp := now.AddDate(0, 0, -acc.daysAgo).UnixMilli()
		details := fmt.Sprintf(`{"provider": "%s"}`, acc.provider)
		_, err := db.ExecContext(ctx, `
			INSERT INTO audit_logs (timestamp, user_id, username, action, resource_type, resource_name, details, ip_address, user_agent, success)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			timestamp, acc.userID, acc.username, ActionAccountCreated, ResourceTypeUser, acc.username, details,
			acc.ip, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/119.0.0.0", true)
		if err != nil {
			return fmt.Errorf("failed to insert account creation: %w", err)
		}
	}

	// Password resets
	passwordResets := []struct {
		daysAgo  int
		userID   uint64
		username string
		ip       string
	}{
		{7, 200001, "alice_d1a2b3", "192.168.1.50"},
		{2, 300002, "eve_l4m5n6", "172.16.0.25"},
	}

	for _, reset := range passwordResets {
		timestamp := now.AddDate(0, 0, -reset.daysAgo).UnixMilli()
		_, err := db.ExecContext(ctx, `
			INSERT INTO audit_logs (timestamp, user_id, username, action, resource_type, resource_name, ip_address, user_agent, success)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			timestamp, reset.userID, reset.username, ActionPasswordReset, ResourceTypeUser, reset.username,
			reset.ip, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0", true)
		if err != nil {
			return fmt.Errorf("failed to insert password reset: %w", err)
		}
	}

	// Add more varied activity over the past 30 days
	if err := createRandomActivity(db, ctx, now); err != nil {
		return fmt.Errorf("failed to create random activity: %w", err)
	}

	return nil
}

func createRandomActivity(db *DB, ctx context.Context, now time.Time) error {
	users := []struct {
		id       uint64
		username string
	}{
		{200001, "alice_d1a2b3"},
		{200002, "bob_c4d5e6"},
		{200003, "charlie_f7g8h9"},
		{300001, "diana_i1j2k3"},
		{300002, "eve_l4m5n6"},
	}

	actions := []string{
		ActionLogin,
		ActionLogout,
		ActionPasswordReset,
		ActionAccessRequested,
	}

	ips := []string{
		"192.168.1.50",
		"10.0.0.100",
		"172.16.0.25",
		"203.0.113.45",
	}

	// Generate 50 random activities over the past 30 days
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 50; i++ {
		user := users[r.Intn(len(users))]
		action := actions[r.Intn(len(actions))]
		ip := ips[r.Intn(len(ips))]

		// Random time in the past 30 days
		daysAgo := r.Intn(30)
		hoursAgo := r.Intn(24)
		minutesAgo := r.Intn(60)

		timestamp := now.AddDate(0, 0, -daysAgo).
			Add(-time.Duration(hoursAgo) * time.Hour).
			Add(-time.Duration(minutesAgo) * time.Minute).
			UnixMilli()

		_, err := db.ExecContext(ctx, `
			INSERT INTO audit_logs (timestamp, user_id, username, action, resource_type, ip_address, user_agent, success)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			timestamp, user.id, user.username, action, ResourceTypeSession, ip,
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0", true)
		if err != nil {
			return fmt.Errorf("failed to insert random activity: %w", err)
		}
	}

	return nil
}
