//go:build windows || linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	servicepkg "github.com/kardianos/service"
)

const (
	serviceName        = "gpipe_client"
	serviceDisplayName = "gpipe client"
	serviceDescription = "go net pipe client"
)

// serviceProgram 把客户端主循环挂到系统服务生命周期里。
type serviceProgram struct {
	common commonArgs

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan error
}

func (p *serviceProgram) Start(_ servicepkg.Service) error {
	prepareRuntime(p.common)
	_ = changeWorkDirToExecutable()

	ctx, cancel := context.WithCancel(context.Background())

	p.mu.Lock()
	p.cancel = cancel
	p.done = make(chan error, 1)
	done := p.done
	p.mu.Unlock()

	// 服务管理器要求 Start 快速返回，因此实际业务循环放到后台协程。
	go func() {
		done <- runCommandContext(ctx, p.common)
	}()
	return nil
}

func (p *serviceProgram) Stop(_ servicepkg.Service) error {
	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}

	select {
	case err := <-done:
		if err == nil || errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case <-time.After(15 * time.Second):
		return fmt.Errorf("service stop timed out")
	}
}

func runServiceCommand(common commonArgs) error {
	if err := validateCommonArgs(common); err != nil {
		return err
	}
	svc, err := newService(common, nil)
	if err != nil {
		return err
	}
	return svc.Run()
}

func installService(args []string) error {
	// 如果用户在 install 命令中提供了自定义参数，则解析并校验，
	// 然后将这些参数拼接到服务命令行中；否则服务启动时不带额外参数，
	// 直接使用二进制内置的嵌入式配置。
	var (
		common     commonArgs
		svcArgs    []string
		hasCustom  = len(args) > 0
	)

	if hasCustom {
		var err error
		common, err = parseCommonArgs(args)
		if err != nil {
			return err
		}
		if err := validateCommonArgs(common); err != nil {
			return err
		}
		svcArgs = buildServiceArgSlice(common)
	} else {
		// 无自定义参数：确认二进制内置了有效配置
		embedded, ok, err := embeddedCommonArgs()
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("no embedded config found and no arguments provided; please provide --server and --key")
		}
		if err := validateCommonArgs(embedded); err != nil {
			return fmt.Errorf("embedded config is invalid: %w", err)
		}
		// 服务启动时只传 run-service 子命令，不带额外参数
		svcArgs = []string{"run-service"}
		common = embedded
	}

	svc, err := newService(common, svcArgs)
	if err != nil {
		return err
	}
	if err := servicepkg.Control(svc, "install"); err != nil {
		return err
	}

	fmt.Printf("%s service (%s) is installed successfully!\n", runtime.GOOS, serviceName)
	switch runtime.GOOS {
	case "windows":
		fmt.Printf("Start the service typing: sc.exe start \"%s\"\n", serviceName)
	case "linux":
		fmt.Printf("Start the service typing: systemctl start %s\n", serviceName)
	}
	return nil
}

func uninstallService() error {
	svc, err := newService(commonArgs{}, nil)
	if err != nil {
		return err
	}
	_ = servicepkg.Control(svc, "stop")
	if err := servicepkg.Control(svc, "uninstall"); err != nil {
		return err
	}

	fmt.Printf("%s service (%s) is uninstalled!\n", runtime.GOOS, serviceName)
	return nil
}

// newService 创建系统服务实例。
// svcArgs 为注册到服务管理器的命令行参数列表；传 nil 时使用默认的参数构建逻辑。
func newService(common commonArgs, svcArgs []string) (servicepkg.Service, error) {
	if svcArgs == nil {
		svcArgs = buildServiceArgSlice(common)
	}
	config := &servicepkg.Config{
		Name:        serviceName,
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		Arguments:   svcArgs,
	}
	return servicepkg.New(&serviceProgram{common: common}, config)
}

func buildServiceArgSlice(common commonArgs) []string {
	args := []string{
		"run-service",
		fmt.Sprintf("--backtrace=%t", common.Backtrace),
		fmt.Sprintf("--server=%s", common.Server),
		fmt.Sprintf("--key=%s", common.Key),
		fmt.Sprintf("--log-level=%s", common.LogLevel),
		fmt.Sprintf("--base-log-level=%s", common.BaseLogLevel),
		fmt.Sprintf("--tls-server-name=%s", common.TLSServerName),
	}
	if common.LogDir != "" {
		args = append(args, fmt.Sprintf("--log-dir=%s", common.LogDir))
	}
	if common.EnableTLS {
		args = append(args, "--enable-tls")
	}
	if common.Insecure {
		args = append(args, "--insecure")
	}
	if common.Quiet {
		args = append(args, "--quiet")
	}
	if common.SSServer != "" {
		args = append(args, fmt.Sprintf("--ss-server=%s", common.SSServer))
	}
	if common.SSMethod != "" {
		args = append(args, fmt.Sprintf("--ss-method=%s", common.SSMethod))
	}
	if common.SSPassword != "" {
		args = append(args, fmt.Sprintf("--ss-password=%s", common.SSPassword))
	}
	return args
}

func changeWorkDirToExecutable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return os.Chdir(filepath.Dir(exe))
}
