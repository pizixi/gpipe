//go:build windows

package upgrade

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	svcmgr "golang.org/x/sys/windows/svc/mgr"
)

const serviceTransitionTimeout = 45 * time.Second

func prepareDetachedCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	if err != nil {
		return err
	}
	if status == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("old client process %d did not exit", pid)
	}
	return nil
}

func startService(name string) error {
	manager, service, err := openWindowsService(name)
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Running {
		return nil
	}
	if status.State == svc.StartPending {
		return waitForWindowsServiceState(service, svc.Running, serviceTransitionTimeout)
	}
	if status.State == svc.StopPending {
		if err := waitForWindowsServiceState(service, svc.Stopped, serviceTransitionTimeout); err != nil {
			return err
		}
	}
	if err := service.Start(); err != nil {
		if current, queryErr := service.Query(); queryErr == nil && current.State == svc.Running {
			return nil
		}
		return fmt.Errorf("start Windows service %q: %w", name, err)
	}
	return waitForWindowsServiceState(service, svc.Running, serviceTransitionTimeout)
}

func stopService(name string) error {
	manager, service, err := openWindowsService(name)
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Stopped {
		return nil
	}
	if status.State != svc.StopPending {
		if _, err := service.Control(svc.Stop); err != nil {
			if current, queryErr := service.Query(); queryErr == nil && current.State == svc.Stopped {
				return nil
			}
			return fmt.Errorf("stop Windows service %q: %w", name, err)
		}
	}
	return waitForWindowsServiceState(service, svc.Stopped, serviceTransitionTimeout)
}

func openWindowsService(name string) (*svcmgr.Mgr, *svcmgr.Service, error) {
	manager, err := svcmgr.Connect()
	if err != nil {
		return nil, nil, fmt.Errorf("connect to Windows service manager: %w", err)
	}
	service, err := manager.OpenService(name)
	if err != nil {
		manager.Disconnect()
		return nil, nil, fmt.Errorf("open Windows service %q: %w", name, err)
	}
	return manager, service, nil
}

func waitForWindowsServiceState(service *svcmgr.Service, expected svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last svc.State
	for time.Now().Before(deadline) {
		status, err := service.Query()
		if err != nil {
			return err
		}
		last = status.State
		if last == expected {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("Windows service state did not reach %d within %s (last state %d)", expected, timeout, last)
}

func syncDirectory(_ string) error { return nil }
func atomicReplace(source, destination string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
