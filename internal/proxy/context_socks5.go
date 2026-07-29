package proxy

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	socks5Version         = 0x05
	socks5AuthNone        = 0x00
	socks5AuthPassword    = 0x02
	socks5AuthUnavailable = 0xff
	socks5CmdTCPConnect   = 0x01
	socks5CmdUDPAssociate = 0x03

	socks5MaxBufferedPayloadBytes = 512 * 1024
)

type socks5Status int

const (
	socks5StatusInit socks5Status = iota
	socks5StatusVerify
	socks5StatusConnect
	socks5StatusConnecting
	socks5StatusRunTCP
	socks5StatusRunUDP
	socks5StatusClosing
)

// Socks5Context 对齐 Rust 中的 Socks5Context 状态机。
type Socks5Context struct {
	status        atomic.Int32
	buffer        []byte
	writer        PeerWriter
	data          *ContextData
	peerAddr      net.Addr
	targetAddr    *TargetAddr
	connectIsTCP  bool
	udpSocket     *net.UDPConn
	udpMu         sync.RWMutex
	udpTargetAddr *net.UDPAddr
	udpClientIP   net.IP
	udpDone       chan struct{}
}

func NewSocks5Context() *Socks5Context {
	ctx := &Socks5Context{}
	ctx.status.Store(int32(socks5StatusInit))
	return ctx
}

func (c *Socks5Context) OnStart(data *ContextData, peerAddr net.Addr, writer PeerWriter) error {
	c.data = data
	c.peerAddr = peerAddr
	c.writer = writer
	return nil
}

func (c *Socks5Context) OnPeerData(data *ContextData, payload []byte) error {
	switch socks5Status(c.status.Load()) {
	case socks5StatusInit, socks5StatusVerify, socks5StatusConnect:
		if len(c.buffer)+len(payload) > socks5MaxBufferedPayloadBytes {
			return fmt.Errorf("socks5 handshake buffer too large")
		}
		c.buffer = append(c.buffer, payload...)
		return c.processBufferedHandshake()
	case socks5StatusConnecting:
		if len(c.buffer)+len(payload) > socks5MaxBufferedPayloadBytes {
			return fmt.Errorf("socks5 pending payload too large")
		}
		c.buffer = append(c.buffer, payload...)
	case socks5StatusRunTCP:
		return c.sendTCPPayload(payload)
	case socks5StatusRunUDP:
		return c.onUDPAssociatePayload(payload)
	case socks5StatusClosing:
		return nil
	}
	return nil
}

func (c *Socks5Context) OnProxyMessage(message ProxyMessage) error {
	switch msg := message.(type) {
	case O2IConnect:
		return c.onConnectReply(msg)
	case O2IRecvData:
		decoded, err := c.data.common.DecodeData(msg.Data)
		if err != nil {
			return err
		}
		dataLen := len(msg.Data)
		return c.writer.Write(decoded, func() {
			c.data.output(I2ORecvDataResult{TunnelID: c.data.tunnelID, ID: msg.ID, DataLen: uint32(dataLen)})
		})
	case O2IRecvDataFrom:
		if c.udpSocket == nil {
			return nil
		}
		udpTargetAddr := c.currentUDPTargetAddr()
		if udpTargetAddr == nil {
			// 中文注释：客户端尚未发来第一包 UDP 数据时，还没有可回写的地址。
			return nil
		}
		decoded, err := c.data.common.DecodeData(msg.Data)
		if err != nil {
			return err
		}
		remote, err := net.ResolveUDPAddr("udp", msg.RemoteAddr)
		if err != nil {
			return err
		}
		target := TargetAddr{IP: remote.IP, Port: uint16(remote.Port)}
		addrBytes, err := target.ToBytes()
		if err != nil {
			return err
		}
		packet := append([]byte{0, 0, 0}, addrBytes...)
		packet = append(packet, decoded...)
		if _, err := c.udpSocket.WriteToUDP(packet, udpTargetAddr); err != nil {
			return err
		}
		c.data.output(I2ORecvDataResult{TunnelID: c.data.tunnelID, ID: msg.ID, DataLen: uint32(len(msg.Data))})
	case O2IDisconnect:
		if c.writer != nil {
			return c.writer.Close()
		}
	}
	return nil
}

func (c *Socks5Context) OnStop(data *ContextData) error {
	if c.udpSocket != nil {
		_ = c.udpSocket.Close()
	}
	if c.udpDone != nil {
		<-c.udpDone
	}
	data.output(I2ODisconnect{TunnelID: data.tunnelID, ID: data.SessionID()})
	return nil
}

func (c *Socks5Context) processBufferedHandshake() error {
	for {
		state := socks5Status(c.status.Load())
		beforeLen := len(c.buffer)
		var err error
		switch state {
		case socks5StatusInit:
			err = c.onInit()
		case socks5StatusVerify:
			err = c.onVerify()
		case socks5StatusConnect:
			err = c.onConnect()
		default:
			return nil
		}
		if err != nil {
			return err
		}
		if socks5Status(c.status.Load()) == state && len(c.buffer) == beforeLen {
			return nil
		}
	}
}

func (c *Socks5Context) ReadyForRead() bool {
	return true
}

func (c *Socks5Context) onInit() error {
	if len(c.buffer) < 3 {
		return nil
	}
	if c.buffer[0] != socks5Version {
		_ = c.writer.Write([]byte{socks5Version, socks5AuthUnavailable}, nil)
		c.status.Store(int32(socks5StatusClosing))
		CloseLater(c.writer, 10*time.Millisecond)
		return nil
	}
	methodCount := int(c.buffer[1])
	if len(c.buffer) < 2+methodCount {
		return nil
	}
	methods := c.buffer[2 : 2+methodCount]
	wantAuth := c.data.authData.Username != "" || c.data.authData.Password != ""
	method := byte(socks5AuthUnavailable)
	for _, candidate := range methods {
		if !wantAuth && candidate == socks5AuthNone {
			method = socks5AuthNone
			break
		}
		if wantAuth && candidate == socks5AuthPassword {
			method = socks5AuthPassword
			break
		}
	}
	_ = c.writer.Write([]byte{socks5Version, method}, nil)
	c.buffer = c.buffer[2+methodCount:]
	if method == socks5AuthUnavailable {
		c.status.Store(int32(socks5StatusClosing))
		CloseLater(c.writer, 10*time.Millisecond)
		return nil
	}
	if method == socks5AuthNone {
		c.status.Store(int32(socks5StatusConnect))
	} else {
		c.status.Store(int32(socks5StatusVerify))
	}
	return nil
}

func (c *Socks5Context) onVerify() error {
	if len(c.buffer) < 5 {
		return nil
	}
	if c.buffer[0] != 0x01 {
		_ = c.writer.Write([]byte{0x01, 0x01}, nil)
		c.status.Store(int32(socks5StatusClosing))
		CloseLater(c.writer, time.Second)
		return nil
	}
	ulen := int(c.buffer[1])
	if len(c.buffer) < 2+ulen+1 {
		return nil
	}
	plenIdx := 2 + ulen
	plen := int(c.buffer[plenIdx])
	if len(c.buffer) < plenIdx+1+plen {
		return nil
	}
	username := string(c.buffer[2 : 2+ulen])
	password := string(c.buffer[plenIdx+1 : plenIdx+1+plen])
	c.buffer = c.buffer[plenIdx+1+plen:]
	if username == c.data.authData.Username && password == c.data.authData.Password {
		_ = c.writer.Write([]byte{0x01, 0x00}, nil)
		c.status.Store(int32(socks5StatusConnect))
		return nil
	}
	_ = c.writer.Write([]byte{0x01, 0x01}, nil)
	c.status.Store(int32(socks5StatusClosing))
	CloseLater(c.writer, time.Second)
	return nil
}

func (c *Socks5Context) onConnect() error {
	if len(c.buffer) < 4 {
		return nil
	}
	ver, cmd, rsv, atyp := c.buffer[0], c.buffer[1], c.buffer[2], c.buffer[3]
	if ver != socks5Version || rsv != 0x00 {
		return c.replyCommandError(0x07)
	}
	target, size, ok, err := ReadTargetAddr(c.buffer[4:], atyp)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	c.buffer = c.buffer[4+size:]
	switch cmd {
	case socks5CmdTCPConnect:
		c.targetAddr = &target
		c.connectIsTCP = true
		c.status.Store(int32(socks5StatusConnecting))
		c.data.output(I2OConnect{
			TunnelID:         c.data.tunnelID,
			ID:               c.data.SessionID(),
			TunnelType:       uint8(TunnelModeSOCKS5),
			IsTCP:            true,
			IsCompressed:     c.data.common.IsCompressed,
			Addr:             target.String(),
			EncryptionMethod: string(c.data.common.Method),
			EncryptionKey:    EncodeKeyToBase64(c.data.common.Key),
			ClientAddr:       c.peerAddr.String(),
		})
	case socks5CmdUDPAssociate:
		c.targetAddr = &target
		c.connectIsTCP = false
		c.status.Store(int32(socks5StatusConnecting))
		c.data.output(I2OConnect{
			TunnelID:         c.data.tunnelID,
			ID:               c.data.SessionID(),
			TunnelType:       uint8(TunnelModeSOCKS5),
			IsTCP:            false,
			IsCompressed:     c.data.common.IsCompressed,
			Addr:             target.String(),
			EncryptionMethod: string(c.data.common.Method),
			EncryptionKey:    EncodeKeyToBase64(c.data.common.Key),
			ClientAddr:       c.peerAddr.String(),
		})
	default:
		return c.replyCommandError(0x07)
	}
	return nil
}

func (c *Socks5Context) onConnectReply(msg O2IConnect) error {
	if !msg.Success {
		return c.replyCommandError(0x04)
	}
	if c.targetAddr == nil {
		return nil
	}
	if c.status.Load() != int32(socks5StatusConnecting) {
		return nil
	}
	if c.connectIsTCP {
		c.status.Store(int32(socks5StatusRunTCP))
		if err := c.writer.Write([]byte{socks5Version, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}, nil); err != nil {
			return err
		}
		pending := c.buffer
		c.buffer = nil
		return c.sendTCPPayload(pending)
	}
	c.buffer = nil
	return c.bindUDPAssociate()
}

func (c *Socks5Context) sendTCPPayload(payload []byte) error {
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

func (c *Socks5Context) onUDPAssociatePayload(payload []byte) error {
	if len(payload) < 11 || c.data == nil {
		return nil
	}
	if payload[0] != 0x00 || payload[1] != 0x00 || payload[2] != 0x00 {
		return nil
	}
	atyp := payload[3]
	target, size, ok, err := ReadTargetAddr(payload[4:], atyp)
	if err != nil || !ok {
		return err
	}
	body := payload[4+size:]
	encoded, err := c.data.common.EncodeDataAndLimit(body)
	if err != nil {
		return err
	}
	c.data.output(I2OSendToData{
		TunnelID:   c.data.tunnelID,
		ID:         c.data.SessionID(),
		Data:       encoded,
		TargetAddr: target.String(),
	})
	return nil
}

func (c *Socks5Context) replyCommandError(code byte) error {
	reply := []byte{socks5Version, code, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	_ = c.writer.Write(reply, nil)
	c.status.Store(int32(socks5StatusClosing))
	CloseLater(c.writer, time.Second)
	return nil
}

func (c *Socks5Context) bindUDPAssociate() error {
	socket, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return c.replyCommandError(0x04)
	}
	c.udpSocket = socket
	c.status.Store(int32(socks5StatusRunUDP))

	if peerTCP, ok := c.peerAddr.(*net.TCPAddr); ok {
		c.udpClientIP = append(net.IP(nil), peerTCP.IP...)
	}

	localAddr := socket.LocalAddr().(*net.UDPAddr)
	reply := []byte{socks5Version, 0x00, 0x00, 0x01, 0, 0, 0, 0, byte(localAddr.Port >> 8), byte(localAddr.Port)}
	if err := c.writer.Write(reply, nil); err != nil {
		return err
	}

	c.udpDone = make(chan struct{})
	go func() {
		defer close(c.udpDone)
		c.readUDPAssociate()
	}()
	return nil
}

func (c *Socks5Context) readUDPAssociate() {
	buf := make([]byte, proxyUDPReadBufferSize)
	for {
		n, addr, err := c.udpSocket.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if !c.acceptUDPClientAddr(addr) {
			continue
		}
		_ = c.onUDPAssociatePayload(append([]byte(nil), buf[:n]...))
	}
}

func (c *Socks5Context) acceptUDPClientAddr(addr *net.UDPAddr) bool {
	if addr == nil {
		return false
	}
	c.udpMu.Lock()
	defer c.udpMu.Unlock()
	if len(c.udpClientIP) > 0 && !c.udpClientIP.IsUnspecified() && !addr.IP.Equal(c.udpClientIP) {
		return false
	}
	if c.udpTargetAddr == nil {
		c.udpTargetAddr = &net.UDPAddr{IP: append(net.IP(nil), addr.IP...), Port: addr.Port, Zone: addr.Zone}
		return true
	}
	return addr.Port == c.udpTargetAddr.Port && addr.Zone == c.udpTargetAddr.Zone && addr.IP.Equal(c.udpTargetAddr.IP)
}

func (c *Socks5Context) currentUDPTargetAddr() *net.UDPAddr {
	c.udpMu.RLock()
	defer c.udpMu.RUnlock()
	if c.udpTargetAddr == nil {
		return nil
	}
	return &net.UDPAddr{IP: append(net.IP(nil), c.udpTargetAddr.IP...), Port: c.udpTargetAddr.Port, Zone: c.udpTargetAddr.Zone}
}
