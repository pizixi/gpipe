package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAppliesSQLitePragmas(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "test.db"))
	database, err := Open("sqlite://" + path + "?mode=rwc")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	var journalMode string
	if err := database.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := database.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}

	var foreignKeys int
	if err := database.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
}
