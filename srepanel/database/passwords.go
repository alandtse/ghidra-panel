package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"

	"go.mkw.re/ghidra-panel/common"
	"golang.org/x/crypto/argon2"
)

func (d *DB) GetUserState(ctx context.Context, ident *common.Identity) (*common.UserState, error) {
	hasPass := true
	username := ident.Username
	err := d.
		QueryRowContext(ctx, "SELECT username FROM passwords WHERE id = ? AND provider = ?", ident.ID, ident.Provider).
		Scan(&username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			hasPass = false
			username = "" // Prevent leaking arbitrary OAuth usernames to the Ghidra repo permission checker
		} else {
			return nil, err
		}
	}
	return &common.UserState{
		Username:    username,
		HasPassword: hasPass,
	}, nil
}

func (d *DB) UsernameExists(ctx context.Context, username string) (exist bool, err error) {
	err = d.
		QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM passwords WHERE username = ?)", username).
		Scan(&exist)
	return
}

func (d *DB) CreateAccount(ctx context.Context, id uint64, username string, password string, provider string) error {
	return d.createAccount(ctx, id, username, password, provider, false)
}

func (d *DB) CreateAccountAsSuperAdmin(ctx context.Context, id uint64, username string, password string, provider string) error {
	return d.createAccount(ctx, id, username, password, provider, true)
}

func (d *DB) createAccount(ctx context.Context, id uint64, username string, password string, provider string, isSuperAdmin bool) error {
	hash, salt, err := hashPassword(password)
	if err != nil {
		return err
	}

	_, err = d.ExecContext(
		ctx,
		`INSERT INTO passwords (id, username, hash, salt, format, provider, is_super_admin) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, username, hash, salt[:], 1, provider, isSuperAdmin,
	)
	return err
}

func (d *DB) IsSuperAdmin(ctx context.Context, id uint64, provider string) (bool, error) {
	var isAdmin bool
	err := d.QueryRowContext(
		ctx,
		`SELECT is_super_admin FROM passwords WHERE id = ? AND provider = ?`,
		id, provider,
	).Scan(&isAdmin)
	if err != nil {
		return false, err
	}
	return isAdmin, nil
}

func (d *DB) CountAccounts(ctx context.Context) (int, error) {
	var count int
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM passwords`).Scan(&count)
	return count, err
}

func hashPassword(password string) ([]byte, [16]byte, error) {
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return nil, salt, err
	}

	// Hash password with Argon2id
	hash := argon2.IDKey([]byte(password), salt[:], 1, 19456, 2, 32)
	return hash, salt, nil
}

func (d *DB) UpdatePassword(ctx context.Context, id uint64, password string, provider string) error {
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return err
	}

	// Hash password with Argon2id
	hash := argon2.IDKey([]byte(password), salt[:], 1, 19456, 2, 32)

	result, err := d.ExecContext(
		ctx,
		`UPDATE passwords SET 
			hash = ?,
			salt = ?,
			format = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND provider = ?`,
		hash, salt[:], 1, id, provider,
	)

	if err != nil {
		return err
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (d *DB) UpdateAccount(ctx context.Context, id uint64, username string, password string, provider string) error {
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return err
	}

	// Hash password with Argon2id
	hash := argon2.IDKey([]byte(password), salt[:], 1, 19456, 2, 32)

	result, err := d.ExecContext(
		ctx,
		`UPDATE passwords SET 
			username = ?,
			hash = ?,
			salt = ?,
			format = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND provider = ?`,
		username, hash, salt[:], 1, id, provider,
	)

	if err != nil {
		return err
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (d *DB) SetUsername(ctx context.Context, id uint64, username string, provider string) error {
	_, err := d.ExecContext(
		ctx,
		`UPDATE passwords SET username = ? WHERE id = ? AND provider = ?`,
		username, id, provider,
	)
	return err
}

// DeleteAccount completely removes a Ghidra user's account and any linked OAuth identities from the database
func (d *DB) DeleteAccount(ctx context.Context, primaryID uint64, primaryProvider string) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Get all linked secondary accounts so we can remove their OAuth identities too
	rows, err := tx.QueryContext(ctx, `
		SELECT secondary_id, secondary_provider FROM account_links
		WHERE primary_id = ? AND primary_provider = ?
	`, primaryID, primaryProvider)
	if err != nil {
		return err
	}

	var secondaries []struct {
		id       uint64
		provider string
	}
	for rows.Next() {
		var s struct {
			id       uint64
			provider string
		}
		if err := rows.Scan(&s.id, &s.provider); err != nil {
			rows.Close()
			return err
		}
		secondaries = append(secondaries, s)
	}
	rows.Close()

	// 2. Delete the secondary OAuth identities
	for _, s := range secondaries {
		if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_identities WHERE id = ? AND provider = ?`, s.id, s.provider); err != nil {
			return err
		}
	}

	// 3. Delete the primary OAuth identity
	resOAuth, err := tx.ExecContext(ctx, `DELETE FROM oauth_identities WHERE id = ? AND provider = ?`, primaryID, primaryProvider)
	if err != nil {
		return err
	}
	affectedOAuth, _ := resOAuth.RowsAffected()

	// 4. Delete the account_links rules
	if _, err := tx.ExecContext(ctx, `DELETE FROM account_links WHERE primary_id = ? AND primary_provider = ?`, primaryID, primaryProvider); err != nil {
		return err
	}

	// 5. Delete the primary panel password account
	result, err := tx.ExecContext(ctx, `DELETE FROM passwords WHERE id = ? AND provider = ?`, primaryID, primaryProvider)
	if err != nil {
		return err
	}

	affectedPass, _ := result.RowsAffected()

	if affectedOAuth == 0 && affectedPass == 0 {
		return fmt.Errorf("user not found or already deleted")
	}

	return tx.Commit()
}
