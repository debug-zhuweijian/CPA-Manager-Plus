package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	return OpenWithOptions(Options{Path: path})
}

func OpenWithOptions(options Options) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(options.Path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", options.Path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := configureConnection(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func configureConnection(db *sql.DB) error {
	pragmas := []string{
		"pragma busy_timeout = 5000",
		"pragma journal_mode = wal",
		"pragma synchronous = normal",
		"pragma foreign_keys = on",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}
	return nil
}
