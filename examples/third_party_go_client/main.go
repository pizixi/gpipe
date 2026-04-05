package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	gclient "github.com/pizixi/gpipe/client"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var (
		server = "tcp://127.0.0.1:8118"
		key    = "demo"

		// 留空表示直连；三个值同时设置时，会通过 Shadowsocks 出站连接服务端。
		ssServer   = ""
		ssMethod   = "chacha20-ietf-poly1305"
		ssPassword = "your-password"
	)

	logger := log.New(os.Stdout, "[third-party-demo] ", log.LstdFlags|log.Lshortfile)
	dial, err := buildOptionalShadowsocksDial(server, ssServer, ssMethod, ssPassword)
	if err != nil {
		logger.Fatal(err)
	}

	if err := gclient.RunContext(ctx, gclient.Options{
		Server: server,
		Key:    key,
		Logger: logger,
		Dial:   dial,
	}); err != nil {
		logger.Fatal(err)
	}
}

func buildOptionalShadowsocksDial(server, ssServer, ssMethod, ssPassword string) (gclient.DialFunc, error) {
	hasSSConfig := ssServer != "" || ssMethod != "" || ssPassword != ""
	if !hasSSConfig {
		return nil, nil
	}
	if ssServer == "" || ssMethod == "" || ssPassword == "" {
		return nil, fmt.Errorf("ssServer, ssMethod and ssPassword must be set together")
	}
	if err := gclient.ValidateStreamDialServerList(server); err != nil {
		return nil, err
	}
	return gclient.NewShadowsocksDialFunc(gclient.SSDialConfig{
		ServerAddr: ssServer,
		Method:     ssMethod,
		Password:   ssPassword,
	})
}
