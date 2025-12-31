package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrations_Up(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Migrations run automatically in Open()

	// Verify tables exist
	tables := []string{
		"passwords",
		"repositories",
		"oauth_identities",
		"audit_logs",
		"schema_migrations",
	}

	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("table %q does not exist after migrations", table)
		} else if err != nil {
			t.Errorf("error checking for table %q: %v", table, err)
		}
	}
}

func TestMigrations_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open once - runs migrations
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database first time: %v", err)
	}
	db.Close()

	// Open again - should handle already-migrated database
	db, err = Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database second time (migrations should be idempotent): %v", err)
	}
	defer db.Close()
}

func TestMigrations_SchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Migrations run automatically in Open()

	// Check migration version
	var version int
	var dirty bool
	err = db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty)
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}

	if version <= 0 {
		t.Errorf("expected positive migration version, got %d", version)
	}

	if dirty {
		t.Error("schema_migrations marked as dirty after successful migration")
	}
}

func TestMigrations_PasswordsTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Migrations run automatically in Open()

	// Verify passwords table has expected columns
	rows, err := db.Query("PRAGMA table_info(passwords)")
	if err != nil {
		t.Fatalf("failed to get table info: %v", err)
	}
	defer rows.Close()

	expectedColumns := map[string]bool{
		"id":             false,
		"username":       false,
		"hash":           false,
		"salt":           false,
		"format":         false,
		"provider":       false,
		"is_super_admin": false,
		// Note: created_at/updated_at are not in schema yet
	}

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}

		if _, exists := expectedColumns[name]; exists {
			expectedColumns[name] = true
		}
	}

	// Check all expected columns were found
	for col, found := range expectedColumns {
		if !found {
			t.Errorf("expected column %q not found in passwords table", col)
		}
	}
}

func TestMigrations_AuditLogsTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Migrations run automatically in Open()

	// Verify audit_logs table has expected columns
	rows, err := db.Query("PRAGMA table_info(audit_logs)")
	if err != nil {
		t.Fatalf("failed to get table info: %v", err)
	}
	defer rows.Close()

	expectedColumns := map[string]bool{
		"id":            false,
		"timestamp":     false,
		"user_id":       false,
		"username":      false,
		"action":        false,
		"resource_type": false,
		"resource_name": false,
		"details":       false,
		"ip_address":    false,
		"user_agent":    false,
		"success":       false,
	}

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}

		if _, exists := expectedColumns[name]; exists {
			expectedColumns[name] = true
		}
	}

	for col, found := range expectedColumns {
		if !found {
			t.Errorf("expected column %q not found in audit_logs table", col)
		}
	}
}

func TestMigrations_RepositoriesTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Migrations run automatically in Open()

	// Verify repositories table exists and has basic structure
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&count)
	if err != nil {
		t.Errorf("failed to query repositories table: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 repositories initially, got %d", count)
	}
}

func TestOpen_CreatesDatabaseFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "new_test.db")

	// Verify file doesn't exist
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatal("database file already exists")
	}

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}
