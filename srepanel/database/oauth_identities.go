package database

import (
	"context"
	"database/sql"
	"errors"

	"go.mkw.re/ghidra-panel/common"
)

// RecordOAuthLogin records or updates an OAuth login
// This is called every time a user logs in via OAuth
func (d *DB) RecordOAuthLogin(ctx context.Context, ident *common.Identity) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO oauth_identities (id, provider, username, first_login, last_login)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id, provider) DO UPDATE SET
			username = excluded.username,
			last_login = CURRENT_TIMESTAMP
	`, ident.ID, ident.Provider, ident.Username)
	return err
}

// GetOAuthIdentity retrieves the provider for a given user ID
// This is the source of truth - we don't trust the JWT token for provider info
func (d *DB) GetOAuthIdentity(ctx context.Context, id uint64) (provider string, username string, err error) {
	err = d.QueryRowContext(ctx, `
		SELECT provider, username 
		FROM oauth_identities 
		WHERE id = ? 
		ORDER BY last_login DESC 
		LIMIT 1
	`, id).Scan(&provider, &username)
	
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	return provider, username, err
}
