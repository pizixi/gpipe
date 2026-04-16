package proxy

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const proxyAuthRequiredResponse = "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"Proxy\"\r\n\r\n"
const badGatewayHeader = "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/html\r\nConnection: close\r\n\r\n"

const (
	httpMaxHeaderBytes          = 64 * 1024
	httpMaxBufferedPayloadBytes = 512 * 1024
)

type httpStatus int

const (
	httpStatusFree httpStatus = iota
	httpStatusConnecting
	httpStatusRunning
	httpStatusInvalid
)

// HTTPContext 对齐 Rust 中的 HttpContext。
type HTTPContext struct {
	status          atomic.Int32
	cache           []byte
	pending         [][]byte
	pendingBytes    int
	writer          PeerWriter
	peerAddr        net.Addr
	data            *ContextData
	isConnectMethod bool
}

func NewHTTPContext() *HTTPContext {
	ctx := &HTTPContext{}
	ctx.status.Store(int32(httpStatusFree))
	return ctx
}

func (c *HTTPContext) OnStart(data *ContextData, peerAddr net.Addr, writer PeerWriter) error {
	c.data = data
	c.peerAddr = peerAddr
	c.writer = writer
	return nil
}

func (c *HTTPContext) OnPeerData(data *ContextData, payload []byte) error {
	switch httpStatus(c.status.Load()) {
	case httpStatusFree:
		c.cache = append(c.cache, payload...)
		headerEnd := bytes.Index(c.cache, []byte("\r\n\r\n"))
		if headerEnd < 0 && len(c.cache) > httpMaxHeaderBytes {
			return fmt.Errorf("http proxy request header too large")
		}
		if headerEnd >= 0 && headerEnd+4 > httpMaxHeaderBytes {
			return fmt.Errorf("http proxy request header too large")
		}
		if headerEnd < 0 {
			return nil
		}
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.cache)))
		if err != nil {
			return nil
		}
		if !c.authorized(req) {
			c.status.Store(int32(httpStatusInvalid))
			_ = c.writer.Write([]byte(proxyAuthRequiredResponse), nil)
			CloseLater(c.writer, time.Second)
			return nil
		}
		host, port, err := resolveProxyTarget(req)
		if err != nil {
			return err
		}

		c.status.Store(int32(httpStatusConnecting))
		if req.Method == http.MethodConnect {
			c.isConnectMethod = true
			c.cache = []byte("HTTP/1.1 200 Connection Established\r\nProxy-Agent: npipe/HTTP/1.1\r\n\r\n")
		} else {
			req.RequestURI = ""
			req.URL.Scheme = ""
			req.URL.Host = ""
			req.Header.Del("Proxy-Connection")
			req.Header.Del("Proxy-Authorization")
			req.Header.Del("Forwarded")
			req.Header.Del("X-Forwarded-For")
			req.Header.Del("X-Forwarded-Host")
			req.Header.Del("X-Forwarded-Proto")
			req.Header.Del("Via")

			var out bytes.Buffer
			if err := req.Write(&out); err != nil {
				return err
			}
			c.cache = out.Bytes()
		}
		if len(c.cache) > httpMaxBufferedPayloadBytes {
			return fmt.Errorf("http proxy initial payload too large")
		}

		data.output(I2OConnect{
			TunnelID:         data.tunnelID,
			ID:               data.SessionID(),
			TunnelType:       uint8(TunnelModeHTTP),
			IsTCP:            true,
			IsCompressed:     data.common.IsCompressed,
			Addr:             net.JoinHostPort(host, port),
			EncryptionMethod: string(data.common.Method),
			EncryptionKey:    EncodeKeyToBase64(data.common.Key),
			ClientAddr:       c.peerAddr.String(),
		})
	case httpStatusConnecting:
		return c.bufferPendingPayload(payload)
	case httpStatusRunning:
		return c.sendUpstreamPayload(payload)
	}
	return nil
}

func (c *HTTPContext) OnProxyMessage(message ProxyMessage) error {
	switch msg := message.(type) {
	case O2IConnect:
		if msg.Success {
			c.status.Store(int32(httpStatusRunning))
			if c.isConnectMethod {
				if err := c.writer.Write(c.cache, nil); err != nil {
					return err
				}
				c.cache = nil
				return c.flushPendingPayloads()
			}
			if err := c.sendUpstreamPayload(c.cache); err != nil {
				return err
			}
			c.cache = nil
			return c.flushPendingPayloads()
		}
		c.status.Store(int32(httpStatusInvalid))
		body := fmt.Sprintf("<html><body><h1>502 Bad Gateway</h1><p>Proxy connection failed: %s</p></body></html>", msg.ErrorInfo)
		_ = c.writer.Write([]byte(badGatewayHeader+body), nil)
		CloseLater(c.writer, time.Second)
	case O2IRecvData:
		decoded, err := c.data.common.DecodeData(msg.Data)
		if err != nil {
			return err
		}
		dataLen := len(msg.Data)
		return c.writer.Write(decoded, func() {
			c.data.output(I2ORecvDataResult{TunnelID: c.data.tunnelID, ID: msg.ID, DataLen: uint32(dataLen)})
		})
	case O2IDisconnect:
		return c.writer.Close()
	}
	return nil
}

func (c *HTTPContext) OnStop(data *ContextData) error {
	data.output(I2ODisconnect{TunnelID: data.tunnelID, ID: data.SessionID()})
	return nil
}

func (c *HTTPContext) ReadyForRead() bool {
	state := httpStatus(c.status.Load())
	return state != httpStatusConnecting && state != httpStatusInvalid
}

func (c *HTTPContext) authorized(req *http.Request) bool {
	if c.data.authData.Username == "" || c.data.authData.Password == "" {
		return true
	}
	value := req.Header.Get("Proxy-Authorization")
	if !strings.HasPrefix(value, "Basic ") {
		return false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "Basic "))
	if err != nil {
		return false
	}
	parts := strings.SplitN(string(raw), ":", 2)
	return len(parts) == 2 && parts[0] == c.data.authData.Username && parts[1] == c.data.authData.Password
}

func resolveProxyTarget(req *http.Request) (string, string, error) {
	if req.Method == http.MethodConnect {
		authority := strings.TrimSpace(req.Host)
		if authority == "" {
			switch {
			case req.URL != nil && req.URL.Host != "":
				authority = req.URL.Host
			case req.URL != nil && req.URL.Opaque != "":
				authority = req.URL.Opaque
			default:
				authority = req.RequestURI
			}
		}
		authority = strings.TrimPrefix(authority, "//")
		host, port, err := net.SplitHostPort(authority)
		if err == nil {
			return host, port, nil
		}
		if authority == "" {
			return "", "", fmt.Errorf("parse http connect authority error")
		}
		return authority, "443", nil
	}

	targetURL := req.URL
	if targetURL == nil {
		return "", "", fmt.Errorf("parse http path error")
	}
	if !targetURL.IsAbs() {
		path := req.RequestURI
		if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
			path = "http://" + req.Host + path
		}
		parsed, err := url.Parse(path)
		if err != nil {
			return "", "", err
		}
		targetURL = parsed
	}
	host := targetURL.Hostname()
	port := targetURL.Port()
	if port == "" {
		port = "80"
		if strings.EqualFold(targetURL.Scheme, "https") {
			port = "443"
		}
	}
	if host == "" {
		return "", "", fmt.Errorf("parse http host error")
	}
	return host, port, nil
}

func (c *HTTPContext) sendUpstreamPayload(payload []byte) error {
	if len(payload) == 0 || c.data == nil {
		return nil
	}
	encoded, err := c.data.common.EncodeDataAndLimit(payload)
	if err != nil {
		return err
	}
	c.data.output(I2OSendData{TunnelID: c.data.tunnelID, ID: c.data.SessionID(), Data: encoded})
	return nil
}

func (c *HTTPContext) bufferPendingPayload(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	if c.pendingBytes+len(payload) > httpMaxBufferedPayloadBytes {
		return fmt.Errorf("http proxy buffered payload too large")
	}
	c.pending = append(c.pending, append([]byte(nil), payload...))
	c.pendingBytes += len(payload)
	return nil
}

func (c *HTTPContext) flushPendingPayloads() error {
	for _, payload := range c.pending {
		if err := c.sendUpstreamPayload(payload); err != nil {
			return err
		}
	}
	c.pending = nil
	c.pendingBytes = 0
	return nil
}
