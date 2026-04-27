package store

import (
	"database/sql"
	"strings"

	"github.com/pizixi/gpipe/internal/model"
)

const defaultShadowsocksMethod = "chacha20-ietf-poly1305"

// ClientBuildSettingsStore 负责读写客户端生成设置。
// 当前表设计为单例，只使用 id=1 这一行。
type ClientBuildSettingsStore struct {
	db *sql.DB
}

// NewClientBuildSettingsStore 创建设置存储对象。
func NewClientBuildSettingsStore(db *sql.DB) *ClientBuildSettingsStore {
	return &ClientBuildSettingsStore{db: db}
}

// Get 读取当前设置；如果数据库里还没有记录，则返回默认值。
func (s *ClientBuildSettingsStore) Get() (model.ClientBuildSettings, error) {
	row := s.db.QueryRow(`
		SELECT server, enable_tls, tls_server_name, use_shadowsocks, ss_server, ss_method, ss_password
		FROM client_build_settings
		WHERE id = 1`)

	settings, err := scanClientBuildSettings(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return defaultClientBuildSettings(), nil
		}
		return model.ClientBuildSettings{}, err
	}
	return normalizeClientBuildSettings(settings), nil
}

// GetForPlayer 读取玩家专属生成配置；未配置时回退到后台全局客户端设置。
func (s *ClientBuildSettingsStore) GetForPlayer(playerID uint32) (model.ClientBuildSettings, bool, error) {
	row := s.db.QueryRow(`
		SELECT server, enable_tls, tls_server_name, use_shadowsocks, ss_server, ss_method, ss_password
		FROM player_client_build_settings
		WHERE player_id = ?`,
		playerID,
	)

	settings, err := scanClientBuildSettings(row)
	if err != nil {
		if err == sql.ErrNoRows {
			settings, err := s.Get()
			return settings, false, err
		}
		return model.ClientBuildSettings{}, false, err
	}
	return normalizeClientBuildSettings(settings), true, nil
}

// Save 以 upsert 方式保存当前设置，保证后台始终只有一份有效配置。
func (s *ClientBuildSettingsStore) Save(settings model.ClientBuildSettings) error {
	settings = normalizeClientBuildSettings(settings)
	_, err := s.db.Exec(`
		INSERT INTO client_build_settings(
			id,
			server,
			enable_tls,
			tls_server_name,
			use_shadowsocks,
			ss_server,
			ss_method,
			ss_password
		)
		VALUES(1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			server = excluded.server,
			enable_tls = excluded.enable_tls,
			tls_server_name = excluded.tls_server_name,
			use_shadowsocks = excluded.use_shadowsocks,
			ss_server = excluded.ss_server,
			ss_method = excluded.ss_method,
			ss_password = excluded.ss_password`,
		settings.Server,
		boolToInt(settings.EnableTLS),
		settings.TLSServerName,
		boolToInt(settings.UseShadowsocks),
		settings.SSServer,
		settings.SSMethod,
		settings.SSPassword,
	)
	return err
}

// SaveForPlayer 保存玩家专属生成配置。后续为该玩家生成客户端时会优先使用这份配置。
func (s *ClientBuildSettingsStore) SaveForPlayer(playerID uint32, settings model.ClientBuildSettings) error {
	settings = normalizeClientBuildSettings(settings)
	_, err := s.db.Exec(`
		INSERT INTO player_client_build_settings(
			player_id,
			server,
			enable_tls,
			tls_server_name,
			use_shadowsocks,
			ss_server,
			ss_method,
			ss_password
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(player_id) DO UPDATE SET
			server = excluded.server,
			enable_tls = excluded.enable_tls,
			tls_server_name = excluded.tls_server_name,
			use_shadowsocks = excluded.use_shadowsocks,
			ss_server = excluded.ss_server,
			ss_method = excluded.ss_method,
			ss_password = excluded.ss_password`,
		playerID,
		settings.Server,
		boolToInt(settings.EnableTLS),
		settings.TLSServerName,
		boolToInt(settings.UseShadowsocks),
		settings.SSServer,
		settings.SSMethod,
		settings.SSPassword,
	)
	return err
}

type clientBuildSettingsScanner interface {
	Scan(dest ...any) error
}

func scanClientBuildSettings(scanner clientBuildSettingsScanner) (model.ClientBuildSettings, error) {
	var (
		settings       model.ClientBuildSettings
		enableTLS      int
		useShadowsocks int
	)
	if err := scanner.Scan(
		&settings.Server,
		&enableTLS,
		&settings.TLSServerName,
		&useShadowsocks,
		&settings.SSServer,
		&settings.SSMethod,
		&settings.SSPassword,
	); err != nil {
		return model.ClientBuildSettings{}, err
	}
	settings.EnableTLS = enableTLS == 1
	settings.UseShadowsocks = useShadowsocks == 1
	return settings, nil
}

// defaultClientBuildSettings 提供首次启动时的默认配置。
func defaultClientBuildSettings() model.ClientBuildSettings {
	return model.ClientBuildSettings{
		SSMethod: defaultShadowsocksMethod,
	}
}

// normalizeClientBuildSettings 统一裁剪空白、补默认值，并在未启用 SS 时清空相关字段。
func normalizeClientBuildSettings(settings model.ClientBuildSettings) model.ClientBuildSettings {
	settings.Server = strings.TrimSpace(settings.Server)
	settings.TLSServerName = strings.TrimSpace(settings.TLSServerName)
	settings.SSServer = strings.TrimSpace(settings.SSServer)
	settings.SSMethod = strings.TrimSpace(settings.SSMethod)
	settings.SSPassword = strings.TrimSpace(settings.SSPassword)
	if settings.SSMethod == "" {
		settings.SSMethod = defaultShadowsocksMethod
	}
	if !settings.UseShadowsocks {
		settings.SSServer = ""
		settings.SSMethod = defaultShadowsocksMethod
		settings.SSPassword = ""
	}
	return settings
}
