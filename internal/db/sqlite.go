package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(databaseURL string) (*sql.DB, error) {
	driverName, dsn, err := normalizeSQLiteDSN(databaseURL)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := configureSQLite(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func normalizeSQLiteDSN(databaseURL string) (string, string, error) {
	switch {
	case strings.HasPrefix(databaseURL, "sqlite://"):
		return "sqlite", "file:" + strings.TrimPrefix(databaseURL, "sqlite://"), nil
	case strings.HasPrefix(databaseURL, "file:"):
		return "sqlite", databaseURL, nil
	default:
		return "", "", fmt.Errorf("only sqlite database_url is supported in gpipe, got %q", databaseURL)
	}
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS user (
			id INTEGER PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			create_time TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tunnel (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			sender INTEGER NOT NULL,
			receiver INTEGER NOT NULL,
			description TEXT NOT NULL,
			tunnel_type INTEGER NOT NULL,
			password TEXT NOT NULL,
			username TEXT NOT NULL,
			is_compressed INTEGER NOT NULL,
			custom_mapping TEXT NOT NULL,
			encryption_method TEXT NOT NULL
		)`,
		// 单行配置表，保存后台“客户端设置”页面的生成参数。
		`CREATE TABLE IF NOT EXISTS client_build_settings (
			id INTEGER PRIMARY KEY,
			server TEXT NOT NULL,
			enable_tls INTEGER NOT NULL,
			tls_server_name TEXT NOT NULL,
			use_shadowsocks INTEGER NOT NULL,
			ss_server TEXT NOT NULL,
			ss_method TEXT NOT NULL,
			ss_password TEXT NOT NULL
		)`,
		// 玩家级客户端生成配置。未配置时生成逻辑回退到 client_build_settings 的全局默认值。
		`CREATE TABLE IF NOT EXISTS player_client_build_settings (
			player_id INTEGER PRIMARY KEY,
			server TEXT NOT NULL,
			enable_tls INTEGER NOT NULL,
			tls_server_name TEXT NOT NULL,
			use_shadowsocks INTEGER NOT NULL,
			ss_server TEXT NOT NULL,
			ss_method TEXT NOT NULL,
			ss_password TEXT NOT NULL,
			FOREIGN KEY(player_id) REFERENCES user(id) ON DELETE CASCADE
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func configureSQLite(db *sql.DB) error {
	stmts := []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA foreign_keys = ON`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
