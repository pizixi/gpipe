//go:build linux

package upgrade

import (
	"slices"
	"testing"
)

func TestServiceApplyHelperUsesIndependentTransientSystemdUnit(t *testing.T) {
	original := runSystemdTransient
	defer func() { runSystemdTransient = original }()

	var got []string
	runSystemdTransient = func(args ...string) ([]byte, error) {
		got = append([]string(nil), args...)
		return nil, nil
	}
	taskID := "0123456789abcdef0123456789abcdef"
	if err := startApplyHelper("/opt/gpipe/gpipe-client", "/opt/gpipe/.gpipe-update/pending.json", "service", taskID); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--quiet",
		"--collect",
		"--unit=gpipe-client-update-" + taskID,
		"--property=Type=exec",
		"--setenv=" + applyHelperTransientEnv + "=1",
		"--",
		"/opt/gpipe/gpipe-client",
		"apply-update",
		"--state",
		"/opt/gpipe/.gpipe-update/pending.json",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("systemd-run args = %q, want %q", got, want)
	}
}

func TestServiceApplyHelperHandsOffWhenStartedByLegacyClient(t *testing.T) {
	original := runSystemdTransient
	defer func() { runSystemdTransient = original }()
	t.Setenv(applyHelperTransientEnv, "")

	called := false
	runSystemdTransient = func(args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	handedOff, err := ensureApplyHelperIsolation("/opt/gpipe/.gpipe-update/pending.json", ApplyState{
		TaskID: "0123456789abcdef0123456789abcdef",
		Mode:   "service",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !handedOff || !called {
		t.Fatalf("handedOff=%v systemdRunCalled=%v, want both true", handedOff, called)
	}
}

func TestServiceApplyHelperDoesNotHandOffTwice(t *testing.T) {
	original := runSystemdTransient
	defer func() { runSystemdTransient = original }()
	t.Setenv(applyHelperTransientEnv, "1")

	called := false
	runSystemdTransient = func(args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	handedOff, err := ensureApplyHelperIsolation("/opt/gpipe/.gpipe-update/pending.json", ApplyState{
		TaskID: "0123456789abcdef0123456789abcdef",
		Mode:   "service",
	})
	if err != nil {
		t.Fatal(err)
	}
	if handedOff || called {
		t.Fatalf("handedOff=%v systemdRunCalled=%v, want both false", handedOff, called)
	}
}
