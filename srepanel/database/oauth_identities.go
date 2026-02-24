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

// LinkAccount explicitly links a secondary OAuth identity to a primary Ghidra identity
func (d *DB) LinkAccount(ctx context.Context, primaryID uint64, primaryProvider string, secondaryID uint64, secondaryProvider string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO account_links (primary_id, primary_provider, secondary_id, secondary_provider)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(secondary_id, secondary_provider) DO UPDATE SET
			primary_id = excluded.primary_id,
			primary_provider = excluded.primary_provider,
			linked_at = CURRENT_TIMESTAMP
	`, primaryID, primaryProvider, secondaryID, secondaryProvider)
	return err
}

// UnlinkAccount removes the mapping for a secondary identity
func (d *DB) UnlinkAccount(ctx context.Context, secondaryID uint64, secondaryProvider string) error {
	_, err := d.ExecContext(ctx, `
		DELETE FROM account_links
		WHERE secondary_id = ? AND secondary_provider = ?
	`, secondaryID, secondaryProvider)
	return err
}

// ResolvePrimaryIdentity translates an incoming identity ID/Provider to the primary ID/Provider (or returns original if unlinked)
func (d *DB) ResolvePrimaryIdentity(ctx context.Context, id uint64, provider string) (uint64, string, error) {
	var primaryID uint64
	var primaryProvider string
	err := d.QueryRowContext(ctx, `
		SELECT primary_id, primary_provider
		FROM account_links
		WHERE secondary_id = ? AND secondary_provider = ?
	`, id, provider).Scan(&primaryID, &primaryProvider)

	if errors.Is(err, sql.ErrNoRows) {
		return id, provider, nil // Not linked, return original
	}
	return primaryID, primaryProvider, err
}

type LinkedAccount struct {
	ID       uint64
	Provider string
	Username string
}

// GetLinkedAccounts retrieves all secondary OAuth accounts linked to a primary identity
func (d *DB) GetLinkedAccounts(ctx context.Context, primaryID uint64, primaryProvider string) ([]LinkedAccount, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT a.secondary_id, a.secondary_provider, o.username
		FROM account_links a
		LEFT JOIN oauth_identities o ON a.secondary_id = o.id AND a.secondary_provider = o.provider
		WHERE a.primary_id = ? AND a.primary_provider = ?
	`, primaryID, primaryProvider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []LinkedAccount
	for rows.Next() {
		var l LinkedAccount
		var username sql.NullString
		if err := rows.Scan(&l.ID, &l.Provider, &username); err != nil {
			return nil, err
		}
		if username.Valid {
			l.Username = username.String
		}
		links = append(links, l)
	}
	return links, rows.Err()
}
