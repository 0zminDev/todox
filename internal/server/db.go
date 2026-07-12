package server

import (
	"os"
	"strings"
)

func migrate() error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			email         TEXT UNIQUE NOT NULL,
			name          TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at    INTEGER NOT NULL,
			is_admin      INTEGER NOT NULL DEFAULT 0,
			banned        INTEGER NOT NULL DEFAULT 0,
			last_ip       TEXT NOT NULL DEFAULT ''
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
		CREATE TABLE IF NOT EXISTS banned_ips (
			ip         TEXT PRIMARY KEY,
			reason     TEXT NOT NULL DEFAULT '',
			banned_by  INTEGER REFERENCES users(id) ON DELETE SET NULL,
			created_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS lists (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name       TEXT NOT NULL,
			position   REAL NOT NULL,
			created_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS boards (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name       TEXT NOT NULL,
			position   REAL NOT NULL,
			created_at INTEGER NOT NULL
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
		`ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN banned INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN last_ip TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE todos ADD COLUMN list_id INTEGER REFERENCES lists(id) ON DELETE CASCADE`,
		`ALTER TABLE todos ADD COLUMN position REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE lists ADD COLUMN board_id INTEGER REFERENCES boards(id) ON DELETE CASCADE`,
	} {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}

	// These columns are added above via ALTER, so the indexes on them can't
	// live in the CREATE TABLE block for fresh databases.
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_todos_list_position ON todos(list_id, position)`,
		`CREATE INDEX IF NOT EXISTS idx_lists_board_position ON lists(board_id, position)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return promoteAdmins()
}

// promoteAdmins grants admin access to every existing user whose email is
// listed in the comma-separated ADMIN_EMAILS environment variable. Runs on
// every startup, so promoting an already-registered user is just an env var
// change plus a restart. New registrations are checked separately at signup
// time via isAdminEmail, since this only runs once at startup.
func promoteAdmins() error {
	for _, email := range adminEmails() {
		if _, err := db.Exec(`UPDATE users SET is_admin = 1 WHERE email = ?`, email); err != nil {
			return err
		}
	}
	return nil
}

func adminEmails() []string {
	raw := os.Getenv("ADMIN_EMAILS")
	if raw == "" {
		return nil
	}
	var emails []string
	for _, email := range strings.Split(raw, ",") {
		if email = strings.TrimSpace(email); email != "" {
			emails = append(emails, email)
		}
	}
	return emails
}

func isAdminEmail(email string) bool {
	for _, e := range adminEmails() {
		if e == email {
			return true
		}
	}
	return false
}
