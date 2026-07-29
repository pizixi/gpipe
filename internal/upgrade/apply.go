package upgrade

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	targetActivationTimeout = 45 * time.Second
	targetRollbackTimeout   = 45 * time.Second
)

func RunApplyHelper(statePath string) error {
	var state ApplyState
	if err := readJSON(statePath, &state); err != nil {
		return fmt.Errorf("read update state: %w", err)
	}
	if state.Target == "" || state.Candidate == "" || state.Backup == "" || state.ParentPID <= 0 {
		return errors.New("invalid update state")
	}
	// A helper started by a legacy systemd service is still inside that
	// service's cgroup. Hand it off before hashing the (potentially large)
	// candidate: the old service exits immediately after spawning us and
	// KillMode=control-group would otherwise win this race and kill the helper.
	handedOff, err := ensureApplyHelperIsolation(statePath, state)
	if err != nil {
		return recoverBeforeSwitch(statePath, state, fmt.Errorf("isolate upgrade helper from old service: %w", err))
	}
	if handedOff {
		return nil
	}
	if digest, err := fileSHA256(state.Candidate); err != nil || !strings.EqualFold(digest, state.ExpectedSHA256) {
		if err != nil {
			return recoverBeforeSwitch(statePath, state, err)
		}
		return recoverBeforeSwitch(statePath, state, errors.New("candidate checksum changed before apply"))
	}
	if err := waitForProcessExit(state.ParentPID, 45*time.Second); err != nil {
		recordApplyFailure(state, err)
		return err
	}
	if state.Mode == "service" {
		// A service manager may have an automatic restart policy. Explicitly
		// stop the unit after the old PID exits so it cannot race file switching.
		if err := stopService(state.ServiceName); err != nil {
			return recoverBeforeSwitch(statePath, state, fmt.Errorf("stop old service before update: %w", err))
		}
	}
	if err := replaceTarget(state); err != nil {
		recordApplyFailure(state, err)
		if state.Mode == "service" {
			_ = startService(state.ServiceName)
		} else if _, statErr := os.Stat(state.Target); statErr == nil {
			cmd := exec.Command(state.Target, state.RestartArgs...)
			prepareDetachedCommand(cmd)
			_ = cmd.Start()
		}
		clearPendingState(statePath)
		return err
	}
	process, err := startUpdatedClient(state)
	if err != nil {
		recordApplyFailure(state, err)
		if rollbackErr := rollbackTarget(state, nil); rollbackErr != nil {
			err = fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
			recordApplyFailure(state, err)
		}
		clearPendingState(statePath)
		return err
	}
	if waitForFile(state.HealthPath, 120*time.Second) {
		_ = os.Remove(state.Backup)
		_ = os.Remove(statePath)
		_ = os.Remove(filepath.Join(filepath.Dir(statePath), "download.json"))
		return nil
	}
	message := "new client did not become healthy within 120 seconds"
	recordApplyFailure(state, errors.New(message))
	rollbackErr := rollbackTarget(state, process)
	if rollbackErr != nil {
		message += "; rollback failed: " + rollbackErr.Error()
		recordApplyFailure(state, errors.New(message))
	}
	clearPendingState(statePath)
	return errors.New(message)
}

func recoverBeforeSwitch(statePath string, state ApplyState, cause error) error {
	recordApplyFailure(state, cause)
	if waitErr := waitForProcessExit(state.ParentPID, 45*time.Second); waitErr != nil {
		return fmt.Errorf("%w; %v", cause, waitErr)
	}
	if state.Mode == "service" {
		if err := startService(state.ServiceName); err != nil {
			return fmt.Errorf("%w; restart old service: %v", cause, err)
		}
		clearPendingState(statePath)
		return cause
	}
	cmd := exec.Command(state.Target, state.RestartArgs...)
	prepareDetachedCommand(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w; restart old client: %v", cause, err)
	}
	clearPendingState(statePath)
	return cause
}

func clearPendingState(statePath string) {
	_ = os.Remove(statePath)
	_ = os.Remove(filepath.Join(filepath.Dir(statePath), "download.json"))
}

func replaceTarget(state ApplyState) error {
	// Keep the original target path present until the final atomic replace. This
	// avoids a power-loss window where a service executable would be missing.
	if err := copyFileSync(state.Target, state.Backup, 0o700); err != nil {
		return fmt.Errorf("backup old client: %w", err)
	}
	if err := syncDirectory(filepath.Dir(state.Target)); err != nil {
		_ = os.Remove(state.Backup)
		return fmt.Errorf("persist client backup: %w", err)
	}
	installing := state.Target + ".installing-" + state.TaskID
	if err := copyFileSync(state.Candidate, installing, 0o700); err != nil {
		return fmt.Errorf("stage new client: %w", err)
	}
	if err := retryRenameForState(installing, state.Target, targetActivationTimeout, state); err != nil {
		_ = os.Remove(installing)
		return fmt.Errorf("activate new client: %w", err)
	}
	if err := syncDirectory(filepath.Dir(state.Target)); err != nil {
		failed := state.Target + ".failed-" + state.TaskID
		_ = copyFileSync(state.Target, failed, 0o700)
		_ = retryRenameForState(state.Backup, state.Target, targetRollbackTimeout, state)
		_ = syncDirectory(filepath.Dir(state.Target))
		return fmt.Errorf("persist new client: %w", err)
	}
	return nil
}

func rollbackTarget(state ApplyState, process *os.Process) error {
	if process != nil {
		_ = process.Kill()
		_, _ = process.Wait()
	}
	if state.Mode == "service" {
		if err := stopService(state.ServiceName); err != nil {
			return fmt.Errorf("stop updated service before rollback: %w", err)
		}
	}
	failed := state.Target + ".failed-" + state.TaskID
	_ = copyFileSync(state.Target, failed, 0o700)
	if err := retryRenameForState(state.Backup, state.Target, targetRollbackTimeout, state); err != nil {
		return err
	}
	if state.Mode == "service" {
		return startService(state.ServiceName)
	}
	cmd := exec.Command(state.Target, state.RestartArgs...)
	prepareDetachedCommand(cmd)
	return cmd.Start()
}

func startUpdatedClient(state ApplyState) (*os.Process, error) {
	if state.Mode == "service" {
		return nil, startService(state.ServiceName)
	}
	cmd := exec.Command(state.Target, state.RestartArgs...)
	prepareDetachedCommand(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("restart updated client: %w", err)
	}
	return cmd.Process, nil
}

func copyFileSync(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(destination)
	}
	return err
}

func retryRename(source, destination string, timeout time.Duration) error {
	return retryRenameWithHook(source, destination, timeout, nil)
}

func retryRenameForState(source, destination string, timeout time.Duration, state ApplyState) error {
	if state.Mode != "service" {
		return retryRename(source, destination, timeout)
	}
	return retryRenameWithHook(source, destination, timeout, func() error {
		// A service recovery policy can restart the old executable after the
		// initial stop and race the file switch. Reassert Stopped immediately
		// before every replacement attempt.
		return stopService(state.ServiceName)
	})
}

func retryRenameWithHook(source, destination string, timeout time.Duration, beforeAttempt func() error) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	var hookErr error
	attempts := 0
	for time.Now().Before(deadline) {
		attempts++
		if beforeAttempt != nil {
			hookErr = beforeAttempt()
			if hookErr != nil {
				// 无法确认服务已经停止时，绝不能在 Linux 上依赖 rename 的成功结果：
				// 正在运行的旧进程不会阻止替换路径，可能导致旧版本被误判为新版本健康。
				time.Sleep(250 * time.Millisecond)
				continue
			}
		}
		if err := atomicReplace(source, destination); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr == nil {
		if hookErr != nil {
			return fmt.Errorf("prepare replacement of %q with %q after %d attempts: %w", destination, source, attempts, hookErr)
		}
		lastErr = errors.New("replacement timed out before the first replacement attempt")
	}
	if hookErr != nil {
		return fmt.Errorf("replace %q with %q after %d attempts: %w; last service stop retry: %v", destination, source, attempts, lastErr, hookErr)
	}
	return fmt.Errorf("replace %q with %q after %d attempts: %w", destination, source, attempts, lastErr)
}

func waitForFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func recordApplyFailure(state ApplyState, err error) {
	_ = writeJSONAtomic(state.ResultPath, applyResult{TaskID: state.TaskID, State: "rolled_back", Error: err.Error()}, 0o600)
}
