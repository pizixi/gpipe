package main

import "flag"

// parseCommonArgs 先加载内置配置，再让命令行参数覆盖默认值。
// 这样生成出来的专属客户端既能开箱即用，也保留人工覆写能力。
func parseCommonArgs(args []string) (commonArgs, error) {
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	fs.SetOutput(new(noopWriter))

	common, _, err := embeddedCommonArgs()
	if err != nil {
		return commonArgs{}, err
	}
	var ignoredUsername string
	fs.BoolVar(&common.Backtrace, "backtrace", common.Backtrace, "print backtracking information")
	fs.StringVar(&common.Server, "server", common.Server, "server address")
	fs.StringVar(&common.Key, "key", common.Key, "player key")
	fs.StringVar(&common.Key, "password", common.Key, "deprecated: use --key")
	fs.StringVar(&ignoredUsername, "username", ignoredUsername, "deprecated: ignored")
	fs.BoolVar(&common.EnableTLS, "enable-tls", common.EnableTLS, "enable tls")
	fs.StringVar(&common.TLSServerName, "tls-server-name", common.TLSServerName, "tls server name")
	fs.BoolVar(&common.Insecure, "insecure", common.Insecure, "deprecated: skip certificate verification")
	fs.BoolVar(&common.Quiet, "quiet", common.Quiet, "quiet mode")
	fs.StringVar(&common.CACert, "ca-cert", common.CACert, "deprecated: ca cert path (kept for compatibility)")
	fs.StringVar(&common.LogLevel, "log-level", common.LogLevel, "log level")
	fs.StringVar(&common.BaseLogLevel, "base-log-level", common.BaseLogLevel, "base log level")
	fs.StringVar(&common.LogDir, "log-dir", common.LogDir, "log directory")
	fs.StringVar(&common.SSServer, "ss-server", common.SSServer, "shadowsocks server address")
	fs.StringVar(&common.SSMethod, "ss-method", common.SSMethod, "shadowsocks cipher method")
	fs.StringVar(&common.SSPassword, "ss-password", common.SSPassword, "shadowsocks password")

	if err := fs.Parse(args); err != nil {
		return commonArgs{}, err
	}
	return common, nil
}

// noopWriter 用于吞掉 flag 包默认的帮助输出，保持现有 CLI 行为。
type noopWriter struct{}

func (*noopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
