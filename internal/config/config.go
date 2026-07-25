package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pizixi/gpipe/internal/upgrade"
)

type ServerConfig struct {
	DatabaseURL           string `json:"database_url"`
	ListenAddr            string `json:"listen_addr"`
	IllegalTrafficForward string `json:"illegal_traffic_forward"`
	EnableTLS             bool   `json:"enable_tls"`
	TLSCert               string `json:"tls_cert"`
	TLSKey                string `json:"tls_key"`
	WebBaseDir            string `json:"web_base_dir"`
	WebAddr               string `json:"web_addr"`
	WebUsername           string `json:"web_username"`
	WebPassword           string `json:"web_password"`
	// ClientTemplateDir 指向预构建客户端下载模板目录。
	// 配置后，发布环境可不依赖 Go 工具链直接生成玩家专属客户端。
	ClientTemplateDir string `json:"client_template_dir"`
	// ClientArtifactCacheDir 用于缓存已补丁完成的客户端下载结果。
	ClientArtifactCacheDir string `json:"client_artifact_cache_dir"`
	// ClientLatestVersion is the semantic version embedded in generated client
	// artifacts and advertised by the remote-upgrade UI.
	ClientLatestVersion string `json:"client_latest_version"`
	Quiet               bool   `json:"quiet"`
	LogDir              string `json:"log_dir"`
}

func (c *ServerConfig) Normalize() {
	if c.LogDir == "" {
		c.LogDir = "logs"
	}
}

func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}
	var cfg ServerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	cfg.Normalize()
	if strings.TrimSpace(cfg.ClientLatestVersion) == "" {
		manifestVersion, err := readClientTemplateManifestVersion(cfg.ClientTemplateDir)
		if err != nil {
			return nil, err
		}
		if manifestVersion == "" {
			manifestVersion = "1.0.0"
		}
		cfg.ClientLatestVersion = manifestVersion
	}
	if !upgrade.IsValidVersion(cfg.ClientLatestVersion) {
		return nil, fmt.Errorf("client_latest_version must be a semantic version, got %q", cfg.ClientLatestVersion)
	}
	return &cfg, nil
}

func readClientTemplateManifestVersion(templateDir string) (string, error) {
	if strings.TrimSpace(templateDir) == "" {
		return "", nil
	}
	manifestPath := filepath.Join(templateDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read client template manifest: %w", err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("parse client template manifest: %w", err)
	}
	version := strings.TrimSpace(manifest.Version)
	if version == "" {
		return "", errors.New("client template manifest version is empty")
	}
	return version, nil
}
