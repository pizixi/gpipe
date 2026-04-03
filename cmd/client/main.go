package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pizixi/gpipe/internal/client"
	"github.com/pizixi/gpipe/internal/logx"
)

type commonArgs struct {
	Backtrace     bool
	Server        string
	Key           string
	EnableTLS     bool
	TLSServerName string
	Insecure      bool
	Quiet         bool
	CACert        string
	LogLevel      string
	BaseLogLevel  string
	LogDir        string
	SSServer      string
	SSMethod      string
	SSPassword    string
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	command, args, err := parseCommand(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	switch command {
	case "install":
		common, err := parseCommonArgs(args)
		if err != nil {
			log.Fatal(err)
		}
		if err := installService(common); err != nil {
			log.Fatal(err)
		}
	case "uninstall":
		if err := uninstallService(); err != nil {
			log.Fatal(err)
		}
	case "run-service":
		common, err := parseCommonArgs(args)
		if err != nil {
			log.Fatal(err)
		}
		if err := runServiceCommand(common); err != nil {
			log.Fatal(err)
		}
	case "run":
		common, err := parseCommonArgs(args)
		if err != nil {
			log.Fatal(err)
		}
		if err := runCommand(common); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown command: %s", command)
	}
}

func runCommand(common commonArgs) error {
	return runCommandContext(context.Background(), common)
}

func runCommandContext(ctx context.Context, common commonArgs) error {
	prepareRuntime(common)

	if err := validateCommonArgs(common); err != nil {
		return err
	}

	logger, closer, err := logx.New(common.Quiet, common.LogDir, "client")
	if err != nil {
		return err
	}
	defer closer.Close()

	// 解析可选的 Shadowsocks 自定义拨号；未配置时保持直连。
	dial, err := resolveDial(common)
	if err != nil {
		return err
	}

	app := client.New(client.Options{
		Server:        common.Server,
		Key:           common.Key,
		EnableTLS:     common.EnableTLS,
		TLSServerName: common.TLSServerName,
		Insecure:      common.Insecure,
		CACert:        common.CACert,
		Logger:        logger,
		Dial:          dial,
	})
	return app.RunContext(ctx)
}

// prepareRuntime 把与运行时相关的公共开关提前落到进程环境中。
func prepareRuntime(common commonArgs) {
	if common.Backtrace {
		_ = os.Setenv("GOTRACEBACK", "all")
	}
}

func validateCommonArgs(common commonArgs) error {
	if common.Server == "" || common.Key == "" {
		return fmt.Errorf("server and key are required")
	}
	if _, err := resolveDial(common); err != nil {
		return err
	}
	return nil
}

// resolveDial 根据命令行参数构造可选的自定义拨号器。
// 目前只支持 Shadowsocks，并且只有在完整提供 SS 参数时才启用；
// 如果完全未配置，则返回 nil，客户端按默认直连方式工作。
func resolveDial(common commonArgs) (client.DialFunc, error) {
	hasSSConfig := common.SSServer != "" || common.SSMethod != "" || common.SSPassword != ""
	if !hasSSConfig {
		return nil, nil
	}
	if common.SSServer == "" || common.SSMethod == "" || common.SSPassword == "" {
		return nil, fmt.Errorf("ss-server, ss-method and ss-password must be set together")
	}
	if err := client.ValidateStreamDialServerList(common.Server); err != nil {
		return nil, err
	}

	dialer, err := client.NewSSDialer(client.SSDialConfig{
		ServerAddr: common.SSServer,
		Method:     common.SSMethod,
		Password:   common.SSPassword,
	})
	if err != nil {
		return nil, err
	}
	return dialer.DialContext, nil
}

func parseCommand(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing command")
	}
	command := strings.ToLower(strings.TrimSpace(args[0]))
	return command, args[1:], nil
}
