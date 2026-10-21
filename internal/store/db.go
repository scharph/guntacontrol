// Package store persists the Auswertung history and the audit log in a shared SQLite
// database (pure-Go driver, no cgo).
package store

import (
	"database/sql"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

// OpenDB opens (creating if needed) the shared SQLite database and ensures the schema
// exists. WAL keeps reads non-blocking against the poller's writes; a busy timeout
// avoids "database is locked" under brief contention.
func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// A single writer at a time is simplest and matches our usage (the poller writes).
	db.SetMaxOpenConns(1)

	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA busy_timeout=5000;`,
		`PRAGMA synchronous=NORMAL;`,
		`CREATE TABLE IF NOT EXISTS samples (
			ts      INTEGER NOT NULL,   -- unix millis
			channel TEXT    NOT NULL,
			value   REAL    NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_samples_channel_ts ON samples(channel, ts);`,
		`CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(ts);`,
		`CREATE TABLE IF NOT EXISTS audit (
			id     INTEGER PRIMARY KEY AUTOINCREMENT,
			ts     INTEGER NOT NULL,   -- unix millis
			who    TEXT,
			via    TEXT,
			action TEXT,
			target TEXT,
			value  TEXT,
			result TEXT,
			detail TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit(ts);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}
