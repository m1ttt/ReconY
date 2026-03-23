package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps a sql.DB connection to SQLite.
type DB struct {
	*sql.DB
}

// Open creates or opens a SQLite database at the given path.
// It sets WAL mode, foreign keys, and busy timeout for concurrent access.
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Verify connection works
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// Set connection pool (SQLite is single-writer, but WAL allows concurrent reads)
	sqlDB.SetMaxOpenConns(1)

	return &DB{sqlDB}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}
