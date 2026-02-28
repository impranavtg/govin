package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot find home directory: %w", err)
	}
	dir := filepath.Join(home, ".govin")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create data directory: %w", err)
	}
	dbPath := filepath.Join(dir, "data.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("cannot open database: %w", err)
	}


	DB = db
	return migrate()
}

func migrate() error {
	schema := `
	PRAGMA journal_mode=WAL;
	PRAGMA foreign_keys=ON;

	CREATE TABLE IF NOT EXISTS groups (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL UNIQUE,
		currency   TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS members (
		id         TEXT PRIMARY KEY,
		group_id   TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
		name       TEXT NOT NULL,
		created_at DATETIME DEFAULT (datetime('now')),
		UNIQUE(group_id, name)
	);

	CREATE TABLE IF NOT EXISTS expenses (
		id          TEXT PRIMARY KEY,
		group_id    TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
		description TEXT NOT NULL,
		amount      REAL NOT NULL,
		paid_by     TEXT NOT NULL,
		created_at  DATETIME DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS expense_splits (
		id         TEXT PRIMARY KEY,
		expense_id TEXT NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
		member_id  TEXT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		amount     REAL NOT NULL
	);

	CREATE TABLE IF NOT EXISTS settlements (
		id          TEXT PRIMARY KEY,
		group_id    TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
		from_member TEXT NOT NULL,
		to_member   TEXT NOT NULL,
		amount      REAL NOT NULL,
		settled_at  DATETIME DEFAULT (datetime('now'))
	);
	`
	if _, err := DB.Exec(schema); err != nil {
		return err
	}
	// Safe migration: add currency column for existing DBs (ignored if already present).
	DB.Exec(`ALTER TABLE groups ADD COLUMN currency TEXT NOT NULL DEFAULT ''`)

	// Bot sessions for Telegram bot per-chat active group tracking.
	DB.Exec(`CREATE TABLE IF NOT EXISTS bot_sessions (
		chat_id INTEGER PRIMARY KEY,
		active_group TEXT NOT NULL DEFAULT ''
	)`)
	return nil
}
