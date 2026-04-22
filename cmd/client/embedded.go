package main

import "github.com/pizixi/gpipe/internal/clientbin"

// embeddedClientConfig 默认保持空字符串，模板构建和回退编译都通过 -ldflags -X 注入。
// 这样链接器会把完整字节串直接写进二进制，服务端下载时才能稳定定位并替换占位块。
var embeddedClientConfig string

// embeddedCommonArgs 解析二进制内置的默认参数。
// 当当前文件还是模板占位状态时，返回 ok=false，调用方继续走普通命令行模式。
func embeddedCommonArgs() (commonArgs, bool, error) {
	if clientbin.IsPlaceholderValue(embeddedClientConfig) {
		return commonArgs{}, false, nil
	}
	config, err := clientbin.Decode(embeddedClientConfig)
	if err != nil {
		return commonArgs{}, false, err
	}
	if !config.HasRequiredRuntimeValues() {
		return commonArgs{}, false, nil
	}
	return commonArgs{
		Server:        config.Server,
		Key:           config.Key,
		EnableTLS:     config.EnableTLS,
		TLSServerName: config.TLSServerName,
		SSServer:      config.SSServer,
		SSMethod:      config.SSMethod,
		SSPassword:    config.SSPassword,
		LogLevel:      "info",
		BaseLogLevel:  "error",
	}, true, nil
}
