package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRetryRenameRunsRecoveryHookBeforeEveryAttempt(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "candidate")
	destination := filepath.Join(dir, "client")
	if err := os.WriteFile(destination, []byte("old-client"), 0o700); err != nil {
		t.Fatal(err)
	}
	hookCalls := 0
	err := retryRenameWithHook(source, destination, time.Second, func() error {
		hookCalls++
		if hookCalls == 2 {
			return os.WriteFile(source, []byte("new-client"), 0o700)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if hookCalls != 2 {
		t.Fatalf("recovery hook calls = %d, want one call before each replacement attempt", hookCalls)
	}
}

func TestRetryRenameDoesNotReplaceWhenRecoveryHookFails(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "candidate")
	destination := filepath.Join(dir, "client")
	if err := os.WriteFile(source, []byte("new-client"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old-client"), 0o700); err != nil {
		t.Fatal(err)
	}
	stopErr := errors.New("service is still running")
	err := retryRenameWithHook(source, destination, 10*time.Millisecond, func() error {
		return stopErr
	})
	if !errors.Is(err, stopErr) {
		t.Fatalf("retry error = %v, want service stop error", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old-client" {
		t.Fatalf("destination was replaced despite failed stop hook: %q", data)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("candidate was consumed despite failed stop hook: %v", err)
	}
}

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
