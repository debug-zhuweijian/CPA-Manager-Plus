package sqlite

import (
	"path/filepath"
	"testing"
)

func TestOpenConfiguresSingleConnectionPoolAndPragmas(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("max open connections = %d, want 1", got)
	}

	var busyTimeout int
	if err := db.QueryRow("pragma busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout < 5000 {
		t.Fatalf("busy_timeout = %d, want at least 5000", busyTimeout)
	}

	var journalMode string
	if err := db.QueryRow("pragma journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}
