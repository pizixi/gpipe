package logx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWithoutLogDirDoesNotCreateLogArtifacts(t *testing.T) {
	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	_, closer, err := New(false, "", "client")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}

	for _, path := range []string{
		filepath.Join(workDir, "logs"),
		filepath.Join(workDir, "client.log"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("log artifact %s exists or stat failed: %v", path, err)
		}
	}
}
