package database

import (
	"context"
)

// IsPanelAdmin checks if a user is a panel super admin (independent of Ghidra account)
func (d *DB) IsPanelAdmin(ctx context.Context, id uint64, provider string) (bool, error) {
	var exists bool
	err := d.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM panel_admins WHERE id = ? AND provider = ?)`,
		id, provider,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// GrantPanelAdmin grants panel super admin privileges to a user
func (d *DB) GrantPanelAdmin(ctx context.Context, id uint64, provider string) error {
	_, err := d.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO panel_admins (id, provider) VALUES (?, ?)`,
		id, provider,
	)
	return err
}

// RevokePanelAdmin revokes panel super admin privileges from a user
func (d *DB) RevokePanelAdmin(ctx context.Context, id uint64, provider string) error {
	_, err := d.ExecContext(
		ctx,
		`DELETE FROM panel_admins WHERE id = ? AND provider = ?`,
		id, provider,
	)
	return err
}

// CountPanelAdmins returns the total number of panel admins
func (d *DB) CountPanelAdmins(ctx context.Context) (int, error) {
	var count int
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM panel_admins`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ListPanelAdmins returns all panel admin user IDs and providers
func (d *DB) ListPanelAdmins(ctx context.Context) ([]struct {
	ID       uint64
	Provider string
}, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, provider FROM panel_admins ORDER BY granted_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []struct {
		ID       uint64
		Provider string
	}
	for rows.Next() {
		var admin struct {
			ID       uint64
			Provider string
		}
		if err := rows.Scan(&admin.ID, &admin.Provider); err != nil {
			return nil, err
		}
		admins = append(admins, admin)
	}
	return admins, rows.Err()
}
