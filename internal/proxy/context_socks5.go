package proxy

import (
	"net"
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
)

type socks5Status int

const (
	socks5StatusInit socks5Status = iota
	socks5StatusVerify
	socks5StatusConnect
	socks5StatusConnecting
	socks5StatusRunTCP
	socks5StatusRunUDP
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
	udpTargetAddr *net.UDPAddr
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
	case socks5StatusInit:
		c.buffer = append(c.buffer, payload...)
		return c.onInit()
	case socks5StatusVerify:
		c.buffer = append(c.buffer, payload...)
		return c.onVerify()
	case socks5StatusConnect:
		c.buffer = append(c.buffer, payload...)
		return c.onConnect()
	case socks5StatusRunTCP:
		encoded, err := data.common.EncodeDataAndLimit(payload)
		if err != nil {
			return err
		}
		data.output(I2OSendData{TunnelID: data.tunnelID, ID: data.SessionID(), Data: encoded})
	case socks5StatusRunUDP:
		return c.onUDPAssociatePayload(payload)
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
		if c.udpTargetAddr == nil {
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
		if _, err := c.udpSocket.WriteToUDP(packet, c.udpTargetAddr); err != nil {
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
	data.output(I2ODisconnect{TunnelID: data.tunnelID, ID: data.SessionID()})
	return nil
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
		return c.writer.Write([]byte{socks5Version, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}, nil)
	}
	return c.bindUDPAssociate()
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

	peerUDP, _ := c.peerAddr.(*net.UDPAddr)
	c.udpTargetAddr = peerUDP

	localAddr := socket.LocalAddr().(*net.UDPAddr)
	reply := []byte{socks5Version, 0x00, 0x00, 0x01, 0, 0, 0, 0, byte(localAddr.Port >> 8), byte(localAddr.Port)}
	if err := c.writer.Write(reply, nil); err != nil {
		return err
	}

	go c.readUDPAssociate()
	return nil
}

func (c *Socks5Context) readUDPAssociate() {
	buf := make([]byte, 65535)
	for {
		n, addr, err := c.udpSocket.ReadFromUDP(buf)
		if err != nil {
			return
		}
		c.udpTargetAddr = addr
		_ = c.onUDPAssociatePayload(append([]byte(nil), buf[:n]...))
	}
}
