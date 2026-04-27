package proxy

import (
	"encoding/base64"
	"errors"
	"sync"

	"github.com/pizixi/gpipe/internal/pb"
)

var errFlowControllerClosed = errors.New("flow controller closed")

const (
	proxyTCPReadBufferSize = 65535
	proxyUDPReadBufferSize = 65535
)

// FlowController 用于限制尚未被对端确认的数据量。
type FlowController struct {
	maxBytes int
	cond     *sync.Cond
	inflight int
	closed   bool
}

func NewFlowController(maxBytes int) *FlowController {
	return &FlowController{
		maxBytes: maxBytes,
		cond:     sync.NewCond(&sync.Mutex{}),
	}
}

func (f *FlowController) Acquire(size int) error {
	if size <= 0 {
		return nil
	}
	f.cond.L.Lock()
	defer f.cond.L.Unlock()
	if f.closed {
		return errFlowControllerClosed
	}
	need := size
	if need > f.maxBytes {
		need = f.maxBytes
	}
	for f.inflight+need > f.maxBytes {
		if f.closed {
			return errFlowControllerClosed
		}
		f.cond.Wait()
	}
	if f.closed {
		return errFlowControllerClosed
	}
	f.inflight += need
	return nil
}

func (f *FlowController) Release(size int) {
	if size <= 0 {
		return
	}
	f.cond.L.Lock()
	defer f.cond.L.Unlock()
	f.inflight -= size
	if f.inflight < 0 {
		f.inflight = 0
	}
	f.cond.Broadcast()
}

func (f *FlowController) Close() {
	f.cond.L.Lock()
	defer f.cond.L.Unlock()
	f.closed = true
	f.inflight = 0
	f.cond.Broadcast()
}

// SessionCommonInfo 对齐 Rust 中的压缩/加密和流控配置。
type SessionCommonInfo struct {
	IsCompressed bool
	Method       EncryptionMethod
	Key          []byte
	Flow         *FlowController
}

func NewSessionCommonInfo(isCompressed bool, method EncryptionMethod, key []byte) *SessionCommonInfo {
	return &SessionCommonInfo{
		IsCompressed: isCompressed,
		Method:       method,
		Key:          append([]byte(nil), key...),
		Flow:         NewFlowController(1024 * 1024),
	}
}

func NewSessionCommonInfoFromName(isCompressed bool, methodName string) (*SessionCommonInfo, error) {
	method := ParseEncryptionMethod(methodName)
	key, err := GenerateKey(method)
	if err != nil {
		return nil, err
	}
	return NewSessionCommonInfo(isCompressed, method, key), nil
}

func (c *SessionCommonInfo) Clone() *SessionCommonInfo {
	if c == nil {
		return nil
	}
	return NewSessionCommonInfo(c.IsCompressed, c.Method, c.Key)
}

func (c *SessionCommonInfo) Close() {
	if c == nil || c.Flow == nil {
		return
	}
	c.Flow.Close()
}

// EncodeDataAndLimit 的顺序与 Rust 一致：先压缩，再加密，再申请流控。
func (c *SessionCommonInfo) EncodeDataAndLimit(data []byte) ([]byte, error) {
	out := append([]byte(nil), data...)
	var err error
	if c.IsCompressed {
		out, err = CompressData(out)
		if err != nil {
			return nil, err
		}
	}
	out, err = Encrypt(c.Method, c.Key, out)
	if err != nil {
		return nil, err
	}
	if err := c.Flow.Acquire(len(out)); err != nil {
		return nil, err
	}
	return out, nil
}

// DecodeData 的顺序与 Rust 一致：先解密，再解压。
func (c *SessionCommonInfo) DecodeData(data []byte) ([]byte, error) {
	out, err := Decrypt(c.Method, c.Key, data)
	if err != nil {
		return nil, err
	}
	if c.IsCompressed {
		out, err = DecompressData(out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// BridgeToPB 把代理消息转换成协议消息。
func BridgeToPB(message ProxyMessage) any {
	switch msg := message.(type) {
	case I2OConnect:
		return &pb.I2OConnect{
			TunnelID:         msg.TunnelID,
			SessionID:        msg.ID,
			TunnelType:       uint32(msg.TunnelType),
			IsTCP:            msg.IsTCP,
			IsCompressed:     msg.IsCompressed,
			Addr:             msg.Addr,
			EncryptionMethod: msg.EncryptionMethod,
			EncryptionKey:    msg.EncryptionKey,
			ClientAddr:       msg.ClientAddr,
		}
	case O2IConnect:
		return &pb.O2IConnect{TunnelID: msg.TunnelID, SessionID: msg.ID, Success: msg.Success, ErrorInfo: msg.ErrorInfo}
	case I2OSendData:
		return &pb.I2OSendData{TunnelID: msg.TunnelID, SessionID: msg.ID, Data: msg.Data}
	case I2OSendToData:
		return &pb.I2OSendToData{TunnelID: msg.TunnelID, SessionID: msg.ID, Data: msg.Data, TargetAddr: msg.TargetAddr}
	case O2ISendDataResult:
		return &pb.O2ISendDataResult{TunnelID: msg.TunnelID, SessionID: msg.ID, DataLen: msg.DataLen}
	case O2IRecvData:
		return &pb.O2IRecvData{TunnelID: msg.TunnelID, SessionID: msg.ID, Data: msg.Data}
	case O2IRecvDataFrom:
		return &pb.O2IRecvDataFrom{TunnelID: msg.TunnelID, SessionID: msg.ID, Data: msg.Data, RemoteAddr: msg.RemoteAddr}
	case I2ORecvDataResult:
		return &pb.I2ORecvDataResult{TunnelID: msg.TunnelID, SessionID: msg.ID, DataLen: msg.DataLen}
	case I2ODisconnect:
		return &pb.I2ODisconnect{TunnelID: msg.TunnelID, SessionID: msg.ID}
	case O2IDisconnect:
		return &pb.O2IDisconnect{TunnelID: msg.TunnelID, SessionID: msg.ID}
	default:
		return nil
	}
}

// BridgeFromPB 把协议消息转换成代理消息。
func BridgeFromPB(message any) (ProxyMessage, uint32, bool) {
	switch msg := message.(type) {
	case *pb.I2OConnect:
		return I2OConnect{
			TunnelID:         msg.TunnelID,
			ID:               msg.SessionID,
			TunnelType:       uint8(msg.TunnelType),
			IsTCP:            msg.IsTCP,
			IsCompressed:     msg.IsCompressed,
			Addr:             msg.Addr,
			EncryptionMethod: msg.EncryptionMethod,
			EncryptionKey:    msg.EncryptionKey,
			ClientAddr:       msg.ClientAddr,
		}, msg.TunnelID, true
	case *pb.I2OSendData:
		return I2OSendData{TunnelID: msg.TunnelID, ID: msg.SessionID, Data: msg.Data}, msg.TunnelID, true
	case *pb.I2OSendToData:
		return I2OSendToData{TunnelID: msg.TunnelID, ID: msg.SessionID, Data: msg.Data, TargetAddr: msg.TargetAddr}, msg.TunnelID, true
	case *pb.I2ORecvDataResult:
		return I2ORecvDataResult{TunnelID: msg.TunnelID, ID: msg.SessionID, DataLen: msg.DataLen}, msg.TunnelID, true
	case *pb.I2ODisconnect:
		return I2ODisconnect{TunnelID: msg.TunnelID, ID: msg.SessionID}, msg.TunnelID, true
	case *pb.O2IConnect:
		return O2IConnect{TunnelID: msg.TunnelID, ID: msg.SessionID, Success: msg.Success, ErrorInfo: msg.ErrorInfo}, msg.TunnelID, true
	case *pb.O2IRecvData:
		return O2IRecvData{TunnelID: msg.TunnelID, ID: msg.SessionID, Data: msg.Data}, msg.TunnelID, true
	case *pb.O2IRecvDataFrom:
		return O2IRecvDataFrom{TunnelID: msg.TunnelID, ID: msg.SessionID, Data: msg.Data, RemoteAddr: msg.RemoteAddr}, msg.TunnelID, true
	case *pb.O2IDisconnect:
		return O2IDisconnect{TunnelID: msg.TunnelID, ID: msg.SessionID}, msg.TunnelID, true
	case *pb.O2ISendDataResult:
		return O2ISendDataResult{TunnelID: msg.TunnelID, ID: msg.SessionID, DataLen: msg.DataLen}, msg.TunnelID, true
	default:
		return nil, 0, false
	}
}

func IsI2OMessage(message ProxyMessage) bool {
	switch message.(type) {
	case I2OConnect, I2OSendData, I2OSendToData, I2ORecvDataResult, I2ODisconnect:
		return true
	default:
		return false
	}
}

func EncodeKeyToBase64(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

func DecodeKeyFromBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}
