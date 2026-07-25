package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServerConfigUsesTemplateManifestVersionWhenSettingIsMissing(t *testing.T) {
	templateDir := filepath.Join(t.TempDir(), "client-templates")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "manifest.json"), []byte(`{"version":"0.1.1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "gpipe.json")
	configJSON := fmt.Sprintf(`{"client_template_dir":%q}`, templateDir)
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadServerConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientLatestVersion != "0.1.1" {
		t.Fatalf("client latest version = %q, want 0.1.1", cfg.ClientLatestVersion)
	}
}

func TestLoadServerConfigExplicitVersionOverridesTemplateManifest(t *testing.T) {
	templateDir := filepath.Join(t.TempDir(), "client-templates")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "manifest.json"), []byte(`{"version":"0.1.1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "gpipe.json")
	configJSON := fmt.Sprintf(`{"client_template_dir":%q,"client_latest_version":"2.0.0"}`, templateDir)
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadServerConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientLatestVersion != "2.0.0" {
		t.Fatalf("client latest version = %q, want 2.0.0", cfg.ClientLatestVersion)
	}
}
