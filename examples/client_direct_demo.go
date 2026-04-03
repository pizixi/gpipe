package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	gclient "github.com/pizixi/gpipe/internal/client"
	"github.com/pizixi/gpipe/internal/logx"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var (
		backtrace     = false
		server        = "ws://127.0.0.1:8119"
		key           = "demo"
		enableTLS     = false
		tlsServerName = ""
		insecure      = false
		caCert        = ""
		quiet         = false
		logDir        = "logs"

		ssServer   = "127.0.0.1:8388"
		ssMethod   = "chacha20-ietf-poly1305"
		ssPassword = "your-password"
	)

	if backtrace {
		if err := os.Setenv("GOTRACEBACK", "all"); err != nil {
			log.Fatal(err)
		}
	}

	logger, closer, err := logx.New(quiet, logDir, "client-demo")
	if err != nil {
		log.Fatal(err)
	}
	defer closer.Close()

	dial, err := buildDemoDial(server, ssServer, ssMethod, ssPassword)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := gclient.New(gclient.Options{
		Server:        server,
		Key:           key,
		EnableTLS:     enableTLS,
		TLSServerName: tlsServerName,
		Insecure:      insecure,
		CACert:        caCert,
		Logger:        logger,
		Dial:          dial,
	})
	if err := app.RunContext(ctx); err != nil {
		log.Fatal(err)
	}
}

func buildDemoDial(server, ssServer, ssMethod, ssPassword string) (gclient.DialFunc, error) {
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

	dialer, err := gclient.NewSSDialer(gclient.SSDialConfig{
		ServerAddr: ssServer,
		Method:     ssMethod,
		Password:   ssPassword,
	})
	if err != nil {
		return nil, err
	}
	return dialer.DialContext, nil
}
