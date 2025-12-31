# Database Migrations

Migrations are applied sequentially in order. Each migration has an `up` (apply) and `down` (rollback) file.

## Migration History

1. **1_init** - Initial schema
   - Creates `passwords` table (user credentials)

2. **2_repositories** - Repository management
   - Creates `repositories` table (repository metadata)

3. **3_add_oauth_tables** - Multi-provider OAuth support
   - Creates `oauth_identities` table (tracks which providers users logged in with)
   - Creates `panel_admins` table (super admin management)

4. **4_add_provider** - Provider column for multi-OAuth
   - Adds `provider` column to `passwords` table
   - Defaults to 'discord' for backward compatibility

5. **5_add_super_admin** - Admin system
   - Adds `is_super_admin` column to `passwords` table
   - Defaults to false for existing users

6. **6_add_audit_logs** - Audit logging system
   - Creates `audit_logs` table with indexes
   - Tracks all user actions for security and compliance

## Applying Migrations

The panel automatically applies pending migrations on startup.

**Fresh install:** Runs migrations 1→2→3→4→5→6

**Existing database (v1):** Already has 1+2, runs 3→4→5→6

## Rollback

To rollback the last migration:
```bash
# Manual rollback (if needed)
sqlite3 panel.db < migrations/N_name.down.sql
```

**Note:** Rollback may cause data loss. Always backup before rolling back.
