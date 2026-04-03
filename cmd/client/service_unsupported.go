//go:build !windows && !linux

package main

import "fmt"

func installService(common commonArgs) error {
	_ = common
	return fmt.Errorf("service install is only supported on windows and linux")
}

func uninstallService() error {
	return fmt.Errorf("service uninstall is only supported on windows and linux")
}

func runServiceCommand(common commonArgs) error {
	return runCommand(common)
}
