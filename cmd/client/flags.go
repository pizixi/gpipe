package main

import "flag"

func parseCommonArgs(args []string) (commonArgs, error) {
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	fs.SetOutput(new(noopWriter))

	var common commonArgs
	var ignoredUsername string
	fs.BoolVar(&common.Backtrace, "backtrace", false, "print backtracking information")
	fs.StringVar(&common.Server, "server", "", "server address")
	fs.StringVar(&common.Key, "key", "", "player key")
	fs.StringVar(&common.Key, "password", "", "deprecated: use --key")
	fs.StringVar(&ignoredUsername, "username", "", "deprecated: ignored")
	fs.BoolVar(&common.EnableTLS, "enable-tls", false, "enable tls")
	fs.StringVar(&common.TLSServerName, "tls-server-name", "", "tls server name")
	fs.BoolVar(&common.Insecure, "insecure", false, "deprecated: skip certificate verification")
	fs.BoolVar(&common.Quiet, "quiet", false, "quiet mode")
	fs.StringVar(&common.CACert, "ca-cert", "", "deprecated: ca cert path (kept for compatibility)")
	fs.StringVar(&common.LogLevel, "log-level", "info", "log level")
	fs.StringVar(&common.BaseLogLevel, "base-log-level", "error", "base log level")
	fs.StringVar(&common.LogDir, "log-dir", "logs", "log directory")
	fs.StringVar(&common.SSServer, "ss-server", "", "shadowsocks server address")
	fs.StringVar(&common.SSMethod, "ss-method", "", "shadowsocks cipher method")
	fs.StringVar(&common.SSPassword, "ss-password", "", "shadowsocks password")

	if err := fs.Parse(args); err != nil {
		return commonArgs{}, err
	}
	return common, nil
}

type noopWriter struct{}

func (*noopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
