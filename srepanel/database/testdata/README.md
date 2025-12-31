# Test Data

This directory contains test data fixtures for development and testing.

## Files

### `seed_admin_panel.sql`

SQL fixture for populating a development database with realistic test data for the admin panel.

**Contains:**
- 1 super admin user (admin_super/admin123)
- 7 regular users across different OAuth providers:
  - Discord: alice_d1a2b3, bob_c4d5e6, charlie_f7g8h9
  - GitHub: diana_i1j2k3, eve_l4m5n6
  - Google: frank_o7p8q9
  - GitLab: grace_r1s2t3
- OAuth identity records with login timestamps
- 4 test repositories
- ~30+ audit log entries including:
  - Successful logins
  - Failed login attempts (simulated attacks)
  - Account creations
  - Password resets
  - Access requests/grants
  - Repository operations

**Usage:**

```bash
# Via Makefile (recommended)
make dev-seed

# Manual (SQLite)
sqlite3 test.db < srepanel/database/testdata/seed_admin_panel.sql

# Manual (Windows)
type srepanel\database\testdata\seed_admin_panel.sql | sqlite3 test.db
```

After seeding, start the dev server:
```bash
make dev
```

Then login with:
- **Admin:** admin_super / admin123
- **Users:** alice_d1a2b3/alice123, bob_c4d5e6/bob123, etc.

## Programmatic Test Data

Integration tests use `seedAdminTestData()` function in `web/admin_integration_test.go` for programmatic data generation.

**Benefits:**
- Faster than SQL fixtures
- Self-contained in test code
- Deterministic timestamps relative to test execution
- Easy to modify for specific test scenarios

**Usage in tests:**
```go
func TestYourFeature(t *testing.T) {
    db, cleanup := setupAdminTestDB(t)
    defer cleanup()

    // Database now has 8 users and audit logs
    // Test your feature...
}
```

## When to Use Each Approach

**SQL Fixture (`seed_admin_panel.sql`):**
- ✅ Manual testing via browser
- ✅ Demonstrating admin panel features
- ✅ QA/staging environments
- ✅ Generating screenshots/documentation
- ❌ Automated integration tests (use programmatic instead)

**Programmatic (`seedAdminTestData()`):**
- ✅ Automated integration tests
- ✅ Benchmark tests
- ✅ CI/CD pipelines
- ✅ Tests requiring specific data scenarios
- ❌ Manual testing (SQL is easier)

## Test Data Characteristics

All test data uses:
- **Passwords:** Dummy hashes (not real Argon2id for performance)
- **IPs:** Mix of private (192.168.x.x, 10.x.x.x) and public test IPs
- **User Agents:** Realistic browser/tool signatures
- **Timestamps:** Distributed over last 30 days

**Security Note:** This is TEST data only. Never use in production!
