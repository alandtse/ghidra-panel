-- Track OAuth identities (which provider each user logged in with)
-- This is the source of truth for provider information
CREATE TABLE IF NOT EXISTS oauth_identities (
	id INTEGER NOT NULL,
	provider TEXT NOT NULL,
	username TEXT NOT NULL,
	first_login TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	last_login TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id, provider)
);

-- Panel super admins (independent of Ghidra accounts)
CREATE TABLE IF NOT EXISTS panel_admins (
	id INTEGER NOT NULL,
	provider TEXT NOT NULL,
	granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id, provider)
);
