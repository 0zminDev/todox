package server

import "strings"

func migrate() error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			email         TEXT UNIQUE NOT NULL,
			name          TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at    INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS todos (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			text        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			due_date    TEXT NOT NULL DEFAULT '',
			tag         TEXT NOT NULL DEFAULT '',
			done        INTEGER NOT NULL DEFAULT 0
		);
	`); err != nil {
		return err
	}

	// Additive migrations for databases created before these columns existed.
	// SQLite has no "ADD COLUMN IF NOT EXISTS", so duplicate-column errors are
	// expected and ignored on databases that already have them.
	for _, stmt := range []string{
		`ALTER TABLE todos ADD COLUMN description TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE todos ADD COLUMN due_date TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE todos ADD COLUMN tag TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}

	return nil
}
