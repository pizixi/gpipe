package upgrade

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyHelperRestoresBackupWhenNewBinaryCannotStart(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "client")
	candidate := filepath.Join(dir, "candidate")
	if err := os.WriteFile(target, []byte("old-client"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, []byte("not-an-executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	state := ApplyState{
		TaskID: "0123456789abcdef0123456789abcdef", Target: target, Candidate: candidate,
		Backup: target + ".backup", Mode: "foreground", ParentPID: 999999,
		ExpectedSHA256: SHA256Hex([]byte("not-an-executable")), ExpectedVersion: "1.1.0",
		HealthPath: filepath.Join(dir, "health"), ResultPath: filepath.Join(dir, "result.json"), CreatedAt: time.Now(),
	}
	statePath := filepath.Join(dir, "pending.json")
	if err := writeJSONAtomic(statePath, state, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunApplyHelper(statePath); err == nil {
		t.Fatal("expected invalid new binary to fail")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old-client" {
		t.Fatalf("target = %q, want restored old client", data)
	}
	var result applyResult
	if err := readJSON(state.ResultPath, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != "rolled_back" {
		t.Fatalf("result state = %q", result.State)
	}
}
