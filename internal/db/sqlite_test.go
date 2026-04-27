package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateAddsPlayerLoginInfoColumnsToExistingUserTable(t *testing.T) {
	database, err := sql.Open("sqlite", "file:test_migrate_player_login_info?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(`CREATE TABLE user (
		id INTEGER PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		create_time TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create old user table: %v", err)
	}

	if err := migrate(database); err != nil {
		t.Fatalf("migrate old schema: %v", err)
	}

	columns := map[string]bool{}
	rows, err := database.Query(`PRAGMA table_info("user")`)
	if err != nil {
		t.Fatalf("table info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}
	if !columns["last_online_time"] {
		t.Fatalf("expected column %q to be added", "last_online_time")
	}
}
