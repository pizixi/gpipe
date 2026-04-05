package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	gclient "github.com/pizixi/gpipe/client"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var (
		backtrace     = false
		server        = "tcp://127.0.0.1:8117"
		key           = "demo" // 调整每个客户端的key
		enableTLS     = false
		tlsServerName = ""
		insecure      = false
		caCert        = ""
		quiet         = false

		ssServer   = "127.0.0.1:8388"
		ssMethod   = "chacha20-ietf-poly1305"
		ssPassword = "your-password"
	)

	if backtrace {
		if err := os.Setenv("GOTRACEBACK", "all"); err != nil {
			log.Fatal(err)
		}
	}

	loggerOutput := io.Writer(os.Stdout)
	if quiet {
		loggerOutput = io.Discard
	}
	logger := log.New(loggerOutput, "", log.LstdFlags|log.Lshortfile)

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
