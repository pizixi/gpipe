package proxy

import (
	"net"
	"testing"
	"time"
)

func TestSOCKS5FlushesPayloadPipelinedWithConnectRequest(t *testing.T) {
	var outputs []ProxyMessage
	data := NewContextData(1, TunnelModeSOCKS5, "", func(message ProxyMessage) {
		outputs = append(outputs, message)
	}, NewSessionCommonInfo(false, EncryptionNone, nil), InletAuthData{})
	data.SetSessionID(2)
	ctx := NewSocks5Context()
	if err := ctx.OnStart(data, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}, &testPeerWriter{}); err != nil {
		t.Fatal(err)
	}
	if err := ctx.OnPeerData(data, []byte{5, 1, 0}); err != nil {
		t.Fatal(err)
	}
	requestAndPayload := append([]byte{5, 1, 0, 1, 127, 0, 0, 1, 0, 80}, []byte("hello")...)
	if err := ctx.OnPeerData(data, requestAndPayload); err != nil {
		t.Fatal(err)
	}
	if err := ctx.OnProxyMessage(O2IConnect{TunnelID: 1, ID: 2, Success: true}); err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 2 {
		t.Fatalf("outputs = %d, want connect and pipelined data", len(outputs))
	}
	send, ok := outputs[1].(I2OSendData)
	if !ok {
		t.Fatalf("second output = %T, want I2OSendData", outputs[1])
	}
	decoded, err := data.common.DecodeData(send.Data)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "hello" {
		t.Fatalf("pipelined payload = %q, want hello", decoded)
	}
}

func TestSOCKS5StopsParsingAfterRejectingAuthenticationMethods(t *testing.T) {
	data := NewContextData(1, TunnelModeSOCKS5, "", func(ProxyMessage) {}, NewSessionCommonInfo(false, EncryptionNone, nil), InletAuthData{})
	data.SetSessionID(3)
	ctx := NewSocks5Context()
	writer := &testPeerWriter{}
	if err := ctx.OnStart(data, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}, writer); err != nil {
		t.Fatal(err)
	}

	// 仅提供用户名密码认证，入口未配置认证时应拒绝；尾部字节不能再被当成新握手解析。
	if err := ctx.OnPeerData(data, []byte{5, 1, socks5AuthPassword, 5, 1, socks5AuthNone}); err != nil {
		t.Fatal(err)
	}
	if status := socks5Status(ctx.status.Load()); status != socks5StatusClosing {
		t.Fatalf("status = %v, want closing", status)
	}
	if got := writer.lastWrite(); len(got) != 2 || got[0] != socks5Version || got[1] != socks5AuthUnavailable {
		t.Fatalf("authentication response = %v", got)
	}
}

func TestSOCKS5UDPAssociatePinsFirstClientAddress(t *testing.T) {
	outputs := make(chan ProxyMessage, 4)
	data := NewContextData(3, TunnelModeSOCKS5, "", func(message ProxyMessage) {
		outputs <- message
	}, NewSessionCommonInfo(false, EncryptionNone, nil), InletAuthData{})
	data.SetSessionID(4)
	ctx := NewSocks5Context()
	writer := &testPeerWriter{}
	if err := ctx.OnStart(data, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}, writer); err != nil {
		t.Fatal(err)
	}
	ctx.targetAddr = &TargetAddr{IP: net.IPv4zero, Port: 0}
	ctx.connectIsTCP = false
	ctx.status.Store(int32(socks5StatusConnecting))
	if err := ctx.OnProxyMessage(O2IConnect{TunnelID: 3, ID: 4, Success: true}); err != nil {
		t.Fatal(err)
	}
	defer ctx.OnStop(data)

	reply := writer.lastWrite()
	if len(reply) != 10 {
		t.Fatalf("UDP associate reply length = %d", len(reply))
	}
	port := int(reply[8])<<8 | int(reply[9])
	target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}
	first, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	packet := append([]byte{0, 0, 0, 1, 8, 8, 8, 8, 0, 53}, []byte("dns")...)
	if _, err := first.WriteToUDP(packet, target); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-outputs:
		if _, ok := message.(I2OSendToData); !ok {
			t.Fatalf("first UDP output = %T", message)
		}
	case <-time.After(time.Second):
		t.Fatal("first UDP client packet was not forwarded")
	}
	if _, err := second.WriteToUDP(packet, target); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-outputs:
		t.Fatalf("second UDP source was forwarded: %T", message)
	case <-time.After(100 * time.Millisecond):
	}
}
