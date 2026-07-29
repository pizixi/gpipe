//go:build !windows

package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func prepareDetachedCommand(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

const applyHelperTransientEnv = "GPIPE_UPDATE_TRANSIENT"

var runSystemdTransient = func(args ...string) ([]byte, error) {
	return exec.Command("systemd-run", args...).CombinedOutput()
}

func startApplyHelper(candidate, statePath, mode, taskID string) error {
	if runtime.GOOS == "linux" && mode == "service" {
		return startTransientApplyHelper(candidate, statePath, taskID)
	}

	cmd := exec.Command(candidate, "apply-update", "--state", statePath)
	prepareDetachedCommand(cmd)
	return cmd.Start()
}

func ensureApplyHelperIsolation(statePath string, state ApplyState) (bool, error) {
	if runtime.GOOS != "linux" || state.Mode != "service" || os.Getenv(applyHelperTransientEnv) == "1" {
		return false, nil
	}
	candidate, err := os.Executable()
	if err != nil {
		return false, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
		candidate = resolved
	}
	if err := startTransientApplyHelper(candidate, statePath, state.TaskID); err != nil {
		return false, err
	}
	return true, nil
}

func startTransientApplyHelper(candidate, statePath, taskID string) error {
	unitName := "gpipe-client-update-" + taskID
	output, err := runSystemdTransient(
		"--quiet",
		"--collect",
		"--unit="+unitName,
		"--property=Type=exec",
		"--setenv="+applyHelperTransientEnv+"=1",
		"--",
		candidate,
		"apply-update",
		"--state",
		statePath,
	)
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("start transient systemd upgrade unit %q: %s", unitName, message)
	}
	return nil
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := process.Signal(syscall.Signal(0))
		if err != nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("old client process %d did not exit", pid)
}

func startService(name string) error {
	if err := runSystemctl("start", name); err != nil {
		return err
	}
	return waitForSystemdServiceState(name, true, 45*time.Second)
}

func stopService(name string) error {
	if err := runSystemctl("stop", name); err != nil {
		state, _ := systemdServiceState(name)
		if state == "inactive" || state == "failed" {
			return nil
		}
		return err
	}
	return waitForSystemdServiceState(name, false, 45*time.Second)
}

func runSystemctl(action, name string) error {
	output, err := exec.Command("systemctl", action, name).CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("systemctl %s %q: %s", action, name, message)
}

func waitForSystemdServiceState(name string, running bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := "unknown"
	for time.Now().Before(deadline) {
		state, _ := systemdServiceState(name)
		if state != "" {
			last = state
		}
		if running && state == "active" {
			return nil
		}
		if !running && (state == "inactive" || state == "failed") {
			return nil
		}
		if running && state == "failed" {
			return fmt.Errorf("systemd service %q entered failed state", name)
		}
		time.Sleep(250 * time.Millisecond)
	}
	want := "stopped"
	if running {
		want = "running"
	}
	return fmt.Errorf("systemd service %q did not become %s within %s (last state %s)", name, want, timeout, last)
}

func systemdServiceState(name string) (string, error) {
	output, err := exec.Command("systemctl", "is-active", name).CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func atomicReplace(source, destination string) error { return os.Rename(source, destination) }
