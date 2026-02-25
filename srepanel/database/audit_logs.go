package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Action type constants
const (
	// Authentication
	ActionLogin       = "login"
	ActionLogout      = "logout"
	ActionLoginFailed = "login_failed"

	// Account Management
	ActionAccountCreated = "account_created"
	ActionPasswordReset  = "password_reset"
	ActionAccountUpdated = "account_updated"

	// Access Control
	ActionAccessRequested = "access_requested"
	ActionAccessGranted   = "access_granted"
	ActionAccessRevoked   = "access_revoked"

	// Repository Operations
	ActionRepoCreated = "repo_created"
	ActionRepoDeleted = "repo_deleted"
	ActionRepoUpdated = "repo_updated"

	// System Events
	ActionSystemStartup  = "system_startup"
	ActionSystemShutdown = "system_shutdown"
)

// Resource type constants
const (
	ResourceTypeUser       = "user"
	ResourceTypeRepository = "repository"
	ResourceTypeSession    = "session"
	ResourceTypeSystem     = "system"
)

// Action description map for human-readable display
var ActionDescriptions = map[string]string{
	ActionLogin:           "Logged in",
	ActionLogout:          "Logged out",
	ActionLoginFailed:     "Failed login attempt",
	ActionAccountCreated:  "Created account",
	ActionPasswordReset:   "Reset password",
	ActionAccountUpdated:  "Updated account",
	ActionAccessRequested: "Requested access",
	ActionAccessGranted:   "Granted access",
	ActionAccessRevoked:   "Revoked access",
	ActionRepoCreated:     "Created repository",
	ActionRepoDeleted:     "Deleted repository",
	ActionRepoUpdated:     "Updated repository",
	ActionSystemStartup:   "System started",
	ActionSystemShutdown:  "System shutdown",
}

// GetActionDescription returns a human-readable description for an action
func GetActionDescription(action string) string {
	if desc, ok := ActionDescriptions[action]; ok {
		return desc
	}
	return action
}

// CleanupOldAuditLogs deletes audit logs older than the specified number of days
func (d *DB) CleanupOldAuditLogs(ctx context.Context, olderThanDays int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -olderThanDays).UnixMilli()

	result, err := d.ExecContext(ctx,
		`DELETE FROM audit_logs WHERE timestamp < ?`,
		cutoffTime,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

type AuditLogEntry struct {
	ID           int64
	Timestamp    int64
	UserID       *uint64
	Username     string
	Action       string
	ResourceType string
	ResourceName string
	Details      string
	IPAddress    string
	UserAgent    string
	Success      bool
}

// CreateAuditLog creates a new audit log entry
func (d *DB) CreateAuditLog(ctx context.Context, entry *AuditLogEntry) error {
	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().UnixMilli()
	}

	_, err := d.ExecContext(ctx,
		`INSERT INTO audit_logs (timestamp, user_id, username, action, resource_type, resource_name, details, ip_address, user_agent, success)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.UserID, entry.Username, entry.Action, entry.ResourceType,
		entry.ResourceName, entry.Details, entry.IPAddress, entry.UserAgent, entry.Success,
	)
	return err
}

// CreateAuditLogWithDetails creates an audit log with JSON-encoded details
func (d *DB) CreateAuditLogWithDetails(ctx context.Context, userID *uint64, username, action, resourceType, resourceName, ipAddress, userAgent string, success bool, details map[string]interface{}) error {
	var detailsJSON string
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return err
		}
		detailsJSON = string(b)
	}

	return d.CreateAuditLog(ctx, &AuditLogEntry{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceName: resourceName,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Success:      success,
		Details:      detailsJSON,
	})
}

// GetAuditLogs retrieves audit logs with pagination
func (d *DB) GetAuditLogs(ctx context.Context, limit, offset int) ([]*AuditLogEntry, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, timestamp, user_id, username, action, resource_type, resource_name, details, ip_address, user_agent, success
		 FROM audit_logs
		 ORDER BY timestamp DESC
		 LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetAuditLogsByUser retrieves audit logs for a specific user
func (d *DB) GetAuditLogsByUser(ctx context.Context, userID uint64, limit int) ([]*AuditLogEntry, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, timestamp, user_id, username, action, resource_type, resource_name, details, ip_address, user_agent, success
		 FROM audit_logs
		 WHERE user_id = ?
		 ORDER BY timestamp DESC
		 LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetAuditLogsByAction retrieves audit logs for a specific action type
func (d *DB) GetAuditLogsByAction(ctx context.Context, action string, limit int) ([]*AuditLogEntry, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, timestamp, user_id, username, action, resource_type, resource_name, details, ip_address, user_agent, success
		 FROM audit_logs
		 WHERE action = ?
		 ORDER BY timestamp DESC
		 LIMIT ?`,
		action, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetRecentFailedLogins retrieves recent failed login attempts
func (d *DB) GetRecentFailedLogins(ctx context.Context, limit int) ([]*AuditLogEntry, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, timestamp, user_id, username, action, resource_type, resource_name, details, ip_address, user_agent, success
		 FROM audit_logs
		 WHERE action = ? AND success = 0
		 ORDER BY timestamp DESC
		 LIMIT ?`,
		ActionLoginFailed, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAuditLogs(rows)
}

// GetActiveSessionCount returns the number of unique users who logged in within the specified duration
func (d *DB) GetActiveSessionCount(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT user_id)
		 FROM audit_logs
		 WHERE action = ? AND timestamp >= ? AND success = 1`,
		ActionLogin, since.UnixMilli(),
	).Scan(&count)
	return count, err
}

// GetFailedLoginCount returns the number of failed login attempts within the specified duration
func (d *DB) GetFailedLoginCount(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM audit_logs
		 WHERE action = ? AND timestamp >= ?`,
		ActionLoginFailed, since.UnixMilli(),
	).Scan(&count)
	return count, err
}

// GetLastLoginByUser returns the most recent successful login for a user
func (d *DB) GetLastLoginByUser(ctx context.Context, userID uint64) (*int64, error) {
	var timestamp sql.NullInt64
	err := d.QueryRowContext(ctx,
		`SELECT timestamp
		 FROM audit_logs
		 WHERE user_id = ? AND action = ? AND success = 1
		 ORDER BY timestamp DESC
		 LIMIT 1`,
		userID, ActionLogin,
	).Scan(&timestamp)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !timestamp.Valid {
		return nil, nil
	}

	ts := timestamp.Int64
	return &ts, nil
}

// scanAuditLogs is a helper to scan multiple audit log rows
func scanAuditLogs(rows *sql.Rows) ([]*AuditLogEntry, error) {
	var logs []*AuditLogEntry

	for rows.Next() {
		var log AuditLogEntry
		var userID sql.NullInt64

		err := rows.Scan(
			&log.ID, &log.Timestamp, &userID, &log.Username, &log.Action,
			&log.ResourceType, &log.ResourceName, &log.Details,
			&log.IPAddress, &log.UserAgent, &log.Success,
		)
		if err != nil {
			return nil, err
		}

		if userID.Valid {
			uid := uint64(userID.Int64)
			log.UserID = &uid
		}

		logs = append(logs, &log)
	}

	return logs, rows.Err()
}
