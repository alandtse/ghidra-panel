CREATE TABLE IF NOT EXISTS account_links (
	secondary_id INTEGER NOT NULL,
	secondary_provider TEXT NOT NULL,
	primary_id INTEGER NOT NULL,
	primary_provider TEXT NOT NULL,
	linked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (secondary_id, secondary_provider)
);
