package db

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Database wraps the SQLite connection and provides data access methods.
type Database struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database in dataDir.
func New(dataDir string) (*Database, error) {
	dbPath := filepath.Join(dataDir, "modpacktool.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	// Allow up to 5s waiting on a locked database (needed for concurrent cache writes)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	d := &Database{db: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return d, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS mods (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			authors TEXT NOT NULL DEFAULT '',
			mod_loader TEXT NOT NULL DEFAULT '',
			jar_file_name TEXT NOT NULL DEFAULT '',
			jar_sha1 TEXT NOT NULL DEFAULT '',
			jar_sha512 TEXT NOT NULL DEFAULT '',
			fingerprint INTEGER NOT NULL DEFAULT 0,
			homepage_url TEXT NOT NULL DEFAULT '',
			curseforge_id INTEGER NOT NULL DEFAULT 0,
			modrinth_id TEXT NOT NULL DEFAULT '',
			curseforge_url TEXT NOT NULL DEFAULT '',
			modrinth_url TEXT NOT NULL DEFAULT '',
			icon_url TEXT NOT NULL DEFAULT '',
			provided_ids TEXT NOT NULL DEFAULT '',
			embedding BLOB,
			is_library INTEGER NOT NULL DEFAULT 0,
			library_override INTEGER NOT NULL DEFAULT 0,
			last_scanned TEXT NOT NULL DEFAULT '',
			last_api_check TEXT NOT NULL DEFAULT '',
			online_desc TEXT NOT NULL DEFAULT '',
			loaders TEXT NOT NULL DEFAULT '',
			categories TEXT NOT NULL DEFAULT '',
			project_type TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS dependencies (
			mod_id TEXT NOT NULL,
			dep_mod_id TEXT NOT NULL,
			dep_name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'required',
			satisfied INTEGER NOT NULL DEFAULT 0,
			source TEXT NOT NULL DEFAULT 'manifest',
			PRIMARY KEY (mod_id, dep_mod_id, source)
		)`,
		`CREATE TABLE IF NOT EXISTS config_mappings (
			config_path TEXT NOT NULL,
			mod_id TEXT NOT NULL,
			confidence REAL NOT NULL DEFAULT 0,
			is_manual INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (config_path, mod_id)
		)`,
		`CREATE TABLE IF NOT EXISTS api_cache (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS library_overrides (
			mod_id TEXT PRIMARY KEY,
			override INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS mixins (
			owner_mod_id TEXT NOT NULL,
			mixin_class TEXT NOT NULL,
			target_class TEXT NOT NULL DEFAULT '',
			target_mod_id TEXT NOT NULL DEFAULT '',
			target_members TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (owner_mod_id, mixin_class, target_class)
		)`,
	}

	for _, m := range migrations {
		if _, err := d.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Add columns for existing databases (ignore errors if already present)
	optionalAlters := []string{
		"ALTER TABLE mods ADD COLUMN loaders TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE mods ADD COLUMN categories TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE mods ADD COLUMN project_type TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE mods ADD COLUMN provided_ids TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE mods ADD COLUMN library_override INTEGER NOT NULL DEFAULT 0",
	}
	for _, q := range optionalAlters {
		d.db.Exec(q)
	}

	return nil
}
