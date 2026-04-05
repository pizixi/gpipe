package client

import (
	"context"
	"io"
	"log"

	internalclient "github.com/pizixi/gpipe/internal/client"
)

// DialFunc 通用流式拨号函数类型。
type DialFunc = internalclient.DialFunc

// SSDialConfig 是 Shadowsocks 出站拨号配置。
type SSDialConfig = internalclient.SSDialConfig

// SSDialer 通过 Shadowsocks 服务端建立到目标地址的连接。
type SSDialer = internalclient.SSDialer

// Options 是客户端运行参数。
type Options struct {
	Server        string
	Key           string
	EnableTLS     bool
	TLSServerName string
	Insecure      bool
	CACert        string
	Logger        *log.Logger
	Dial          DialFunc
}

// App 是可被第三方程序直接复用的客户端运行入口。
type App struct {
	inner *internalclient.App
}

// New 创建客户端实例。
func New(opts Options) *App {
	return &App{
		inner: internalclient.New(toInternalOptions(opts)),
	}
}

// Run 运行客户端，直到发生错误或连接被关闭。
func (a *App) Run() error {
	return a.inner.Run()
}

// RunContext 运行客户端，直到发生错误或上下文结束。
func (a *App) RunContext(ctx context.Context) error {
	return a.inner.RunContext(ctx)
}

// Run 直接使用给定参数运行客户端。
func Run(opts Options) error {
	return New(opts).Run()
}

// RunContext 直接使用给定参数运行客户端，并响应上下文取消。
func RunContext(ctx context.Context, opts Options) error {
	return New(opts).RunContext(ctx)
}

// ValidateStreamDialServerList 校验自定义流式拨号可用的服务端地址列表。
func ValidateStreamDialServerList(server string) error {
	return internalclient.ValidateStreamDialServerList(server)
}

// NewSSDialer 创建 Shadowsocks 出站拨号器。
func NewSSDialer(cfg SSDialConfig) (*SSDialer, error) {
	return internalclient.NewSSDialer(cfg)
}

// NewShadowsocksDialFunc 构造可直接注入 Options.Dial 的 Shadowsocks 拨号函数。
func NewShadowsocksDialFunc(cfg SSDialConfig) (DialFunc, error) {
	dialer, err := internalclient.NewSSDialer(cfg)
	if err != nil {
		return nil, err
	}
	return dialer.DialContext, nil
}

func toInternalOptions(opts Options) internalclient.Options {
	logger := opts.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return internalclient.Options{
		Server:        opts.Server,
		Key:           opts.Key,
		EnableTLS:     opts.EnableTLS,
		TLSServerName: opts.TLSServerName,
		Insecure:      opts.Insecure,
		CACert:        opts.CACert,
		Logger:        logger,
		Dial:          opts.Dial,
	}
}
