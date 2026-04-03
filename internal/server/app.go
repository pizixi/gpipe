package server

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pizixi/gpipe/internal/config"
	"github.com/pizixi/gpipe/internal/manager"
	"github.com/pizixi/gpipe/internal/transport"
	"github.com/pizixi/gpipe/internal/web"

	quic "github.com/quic-go/quic-go"
	kcp "github.com/xtaci/kcp-go/v5"
)

const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	httpWriteTimeout      = 30 * time.Second
	httpIdleTimeout       = 60 * time.Second
	httpMaxHeaderBytes    = 1 << 20
)

type App struct {
	Config  *config.ServerConfig
	Logger  *log.Logger
	DB      *sql.DB
	Hub     *Hub
	Runtime *manager.Runtime
}

func NewApp(cfg *config.ServerConfig, logger *log.Logger, db *sql.DB) (*App, error) {
	hub := NewHub(logger)
	hub.illegalForward = cfg.IllegalTrafficForward
	rt, err := manager.NewRuntime(db, hub)
	if err != nil {
		return nil, err
	}
	hub.SetRuntime(rt)
	return &App{
		Config:  cfg,
		Logger:  logger,
		DB:      db,
		Hub:     hub,
		Runtime: rt,
	}, nil
}

func (a *App) RunTCP(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	var tlsCfg *tls.Config
	if a.Config.EnableTLS {
		tlsCfg, err = buildServerTLSConfig(a.Config.TLSCert, a.Config.TLSKey)
		if err != nil {
			_ = ln.Close()
			return err
		}
	}
	a.Logger.Printf("tcp server listening on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		sessionConn := conn
		if tlsCfg != nil {
			sessionConn, err = wrapServerTLSConn(conn, tlsCfg)
			if err != nil {
				a.Logger.Printf("tcp tls handshake failed from %s: %v", conn.RemoteAddr(), err)
				continue
			}
		}
		go a.Hub.NewSession(sessionConn).Run()
	}
}

func (a *App) RunWS(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := transport.WSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go a.Hub.NewSession(transport.NewWSConn(conn)).Run()
	})
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		IdleTimeout:       httpIdleTimeout,
		MaxHeaderBytes:    httpMaxHeaderBytes,
	}
	a.Logger.Printf("ws server listening on %s", addr)
	if a.Config.EnableTLS {
		return server.ListenAndServeTLS(a.Config.TLSCert, a.Config.TLSKey)
	}
	return server.ListenAndServe()
}

func (a *App) RunQUIC(addr string) error {
	if !a.Config.EnableTLS {
		return fmt.Errorf("quic requires tls")
	}
	tlsCfg, err := buildServerTLSConfig(a.Config.TLSCert, a.Config.TLSKey)
	if err != nil {
		return err
	}
	tlsCfg.NextProtos = []string{"npipe", "h3"}
	listener, err := quic.ListenAddr(addr, tlsCfg, nil)
	if err != nil {
		return err
	}
	a.Logger.Printf("quic server listening on %s", addr)
	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}
		go func(connection *quic.Conn) {
			for {
				stream, err := connection.AcceptStream(context.Background())
				if err != nil {
					return
				}
				go a.Hub.NewSession(transport.NewQUICConn(stream, connection.LocalAddr(), connection.RemoteAddr(), func() error {
					return connection.CloseWithError(0, "")
				})).Run()
			}
		}(conn)
	}
}

func (a *App) RunKCP(addr string) error {
	listener, err := kcp.ListenWithOptions(addr, nil, 10, 3)
	if err != nil {
		return err
	}
	transport.TuneKCPListener(a.Logger, listener)
	var tlsCfg *tls.Config
	if a.Config.EnableTLS {
		tlsCfg, err = buildServerTLSConfig(a.Config.TLSCert, a.Config.TLSKey)
		if err != nil {
			_ = listener.Close()
			return err
		}
	}
	a.Logger.Printf("kcp server listening on %s", addr)
	for {
		conn, err := listener.AcceptKCP()
		if err != nil {
			return err
		}
		transport.TuneKCPSession(a.Logger, conn)
		if tlsCfg != nil {
			sessionConn, err := wrapServerTLSConn(conn, tlsCfg)
			if err != nil {
				a.Logger.Printf("kcp tls handshake failed from %s: %v", conn.RemoteAddr(), err)
				continue
			}
			go a.Hub.NewSession(sessionConn).Run()
		} else {
			go a.Hub.NewSession(conn).Run()
		}
	}
}

func (a *App) RunWeb(addr string) error {
	service := web.NewService(a.Config, a.Runtime)
	a.Logger.Printf("web server listening on %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           service.Handler(),
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
		MaxHeaderBytes:    httpMaxHeaderBytes,
	}
	return server.ListenAndServe()
}

func ParseListenAddrs(listenAddr string) []string {
	raw := strings.Split(listenAddr, ",")
	var out []string
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func buildServerTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

var serverTLSHandshakeTimeout = 15 * time.Second

func wrapServerTLSConn(conn net.Conn, cfg *tls.Config) (net.Conn, error) {
	if cfg == nil {
		return conn, nil
	}
	tlsConn := tls.Server(conn, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), serverTLSHandshakeTimeout)
	defer cancel()
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	return tlsConn, nil
}
