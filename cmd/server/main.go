package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"

	"github.com/pizixi/gpipe/internal/config"
	"github.com/pizixi/gpipe/internal/db"
	"github.com/pizixi/gpipe/internal/logx"
	"github.com/pizixi/gpipe/internal/server"
)

const (
	defaultConfigFile = "gpipe.json"
	legacyConfigFile  = "config.json"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	configFile := flag.String("config-file", defaultConfigFile, "config file")
	flag.Parse()

	cfg, err := config.LoadServerConfig(*configFile)
	if err != nil && *configFile == defaultConfigFile && errors.Is(err, os.ErrNotExist) {
		cfg, err = config.LoadServerConfig(legacyConfigFile)
	}
	if err != nil {
		log.Fatal(err)
	}
	logger, closer, err := logx.New(cfg.Quiet, cfg.LogDir, "server")
	if err != nil {
		log.Fatal(err)
	}
	defer closer.Close()

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal(err)
	}
	defer database.Close()

	app, err := server.NewApp(cfg, logger, database)
	if err != nil {
		logger.Fatal(err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	run := func(label string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				select {
				case errCh <- fmt.Errorf("%s server error: %w", label, err):
				default:
				}
			}
		}()
	}
	for _, entry := range server.ParseListenAddrs(cfg.ListenAddr) {
		u, err := url.Parse(entry)
		if err != nil {
			logger.Printf("skip invalid listen addr %q: %v", entry, err)
			continue
		}
		switch u.Scheme {
		case "tcp":
			addr := u.Host
			run("tcp", func() error { return app.RunTCP(addr) })
		case "ws":
			addr := u.Host
			run("ws", func() error { return app.RunWS(addr) })
		case "quic":
			addr := u.Host
			run("quic", func() error { return app.RunQUIC(addr) })
		case "kcp":
			addr := u.Host
			run("kcp", func() error { return app.RunKCP(addr) })
		default:
			logger.Printf("unsupported scheme for now: %s", entry)
		}
	}

	if cfg.WebAddr != "" && cfg.WebUsername != "" && cfg.WebPassword != "" {
		run("web", func() error { return app.RunWeb(cfg.WebAddr) })
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		logger.Print(err)
	case <-done:
	}
}
