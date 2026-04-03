package proxy

import (
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
)

func TestShadowsocksStreamRoundTrip(t *testing.T) {
	writer, err := newShadowsocksStreamWriter(DefaultShadowsocksMethod, "secret")
	if err != nil {
		t.Fatalf("new stream writer: %v", err)
	}
	reader, err := newShadowsocksStreamReader(DefaultShadowsocksMethod, "secret")
	if err != nil {
		t.Fatalf("new stream reader: %v", err)
	}

	targetBytes, err := (TargetAddr{Host: "example.com", Port: 443}).ToBytes()
	if err != nil {
		t.Fatalf("target to bytes: %v", err)
	}

	inputs := [][]byte{
		append(targetBytes, []byte("first-payload")...),
		[]byte("second-payload"),
		make([]byte, shadowsocksMaxPayload+100),
	}
	inputs[2][0] = 'x'
	inputs[2][len(inputs[2])-1] = 'y'

	var network []byte
	for _, input := range inputs {
		packet, err := writer.SealData(input)
		if err != nil {
			t.Fatalf("seal data: %v", err)
		}
		network = append(network, packet...)
	}

	var decoded [][]byte
	for _, chunk := range [][]byte{
		network[:17],
		network[17:83],
		network[83:],
	} {
		part, err := reader.Feed(chunk)
		if err != nil {
			t.Fatalf("feed stream chunk: %v", err)
		}
		decoded = append(decoded, part...)
	}

	expected := [][]byte{
		inputs[0],
		inputs[1],
		inputs[2][:shadowsocksMaxPayload],
		inputs[2][shadowsocksMaxPayload:],
	}

	if len(decoded) != len(expected) {
		t.Fatalf("decoded chunk count = %d, want %d", len(decoded), len(expected))
	}
	for i := range expected {
		if string(decoded[i]) != string(expected[i]) {
			t.Fatalf("decoded chunk %d mismatch", i)
		}
	}
}

func TestShadowsocksPacketRoundTrip(t *testing.T) {
	targetBytes, err := (TargetAddr{Host: "dns.google", Port: 53}).ToBytes()
	if err != nil {
		t.Fatalf("target to bytes: %v", err)
	}

	packet, err := shadowsocksEncryptPacket(DefaultShadowsocksMethod, "secret", append(targetBytes, []byte("hello-udp")...))
	if err != nil {
		t.Fatalf("encrypt packet: %v", err)
	}

	decoded, err := shadowsocksDecryptPacket(DefaultShadowsocksMethod, "secret", packet)
	if err != nil {
		t.Fatalf("decrypt packet: %v", err)
	}

	target, body, err := parseShadowsocksTarget(decoded)
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	if target.String() != "dns.google:53" {
		t.Fatalf("target = %q, want %q", target.String(), "dns.google:53")
	}
	if string(body) != "hello-udp" {
		t.Fatalf("body = %q, want %q", string(body), "hello-udp")
	}
}

func TestShadowsocksInletStartsTCPAndUDP(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	listenAddr := "127.0.0.1:" + freeTCPPort(t)
	inlet := NewInlet(
		logger,
		1,
		TunnelModeShadowsocks,
		listenAddr,
		"",
		NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil),
		InletAuthData{Method: DefaultShadowsocksMethod, Password: "secret"},
		func(ProxyMessage) {},
		"ss-test",
	)
	if err := inlet.Start(); err != nil {
		t.Fatalf("start shadowsocks inlet: %v", err)
	}
	t.Cleanup(func() { _ = inlet.Stop() })

	tcpConn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		t.Fatalf("dial shadowsocks tcp inlet: %v", err)
	}
	_ = tcpConn.Close()

	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		t.Fatalf("resolve shadowsocks udp addr: %v", err)
	}
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatalf("dial shadowsocks udp inlet: %v", err)
	}
	defer udpConn.Close()

	targetBytes, err := (TargetAddr{Host: "example.com", Port: 53}).ToBytes()
	if err != nil {
		t.Fatalf("target to bytes: %v", err)
	}
	packet, err := shadowsocksEncryptPacket(DefaultShadowsocksMethod, "secret", append(targetBytes, []byte("ping")...))
	if err != nil {
		t.Fatalf("encrypt shadowsocks udp packet: %v", err)
	}
	if _, err := udpConn.Write(packet); err != nil {
		t.Fatalf("write shadowsocks udp packet: %v", err)
	}

	if sessionID := waitForUDPPeerSession(t, inlet, udpConn.LocalAddr().String()); sessionID == 0 {
		t.Fatalf("expected shadowsocks udp session to be created")
	}
}

func TestShadowsocksTCPInteroperatesWithGoShadowsocks2(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	listenAddr := "127.0.0.1:" + freeTCPPort(t)
	common := NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil)
	messages := make(chan ProxyMessage, 8)
	inlet := NewInlet(
		logger,
		1,
		TunnelModeShadowsocks,
		listenAddr,
		"",
		common,
		InletAuthData{Method: DefaultShadowsocksMethod, Password: "secret"},
		func(msg ProxyMessage) { messages <- msg },
		"ss-go-shadowsocks2-tcp",
	)
	if err := inlet.Start(); err != nil {
		t.Fatalf("start inlet: %v", err)
	}
	t.Cleanup(func() { _ = inlet.Stop() })

	cipher, err := core.PickCipher(DefaultShadowsocksMethod, nil, "secret")
	if err != nil {
		t.Fatalf("pick cipher: %v", err)
	}
	rawConn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		t.Fatalf("dial inlet: %v", err)
	}
	defer rawConn.Close()
	ssConn := cipher.StreamConn(rawConn)

	targetBytes, err := (TargetAddr{Host: "example.com", Port: 443}).ToBytes()
	if err != nil {
		t.Fatalf("target to bytes: %v", err)
	}
	if _, err := ssConn.Write(append(targetBytes, []byte("hello-ss-tcp")...)); err != nil {
		t.Fatalf("write shadowsocks payload: %v", err)
	}

	connectMsg := waitProxyMessage(t, messages)
	connect, ok := connectMsg.(I2OConnect)
	if !ok {
		t.Fatalf("expected I2OConnect, got %T", connectMsg)
	}
	if connect.Addr != "example.com:443" {
		t.Fatalf("connect addr = %q, want %q", connect.Addr, "example.com:443")
	}

	inlet.Input(O2IConnect{TunnelID: 1, ID: connect.ID, Success: true})

	dataMsg := waitProxyMessage(t, messages)
	sendData, ok := dataMsg.(I2OSendData)
	if !ok {
		t.Fatalf("expected I2OSendData, got %T", dataMsg)
	}
	decoded, err := common.DecodeData(sendData.Data)
	if err != nil {
		t.Fatalf("decode pending payload: %v", err)
	}
	if string(decoded) != "hello-ss-tcp" {
		t.Fatalf("decoded payload = %q, want %q", string(decoded), "hello-ss-tcp")
	}

	replyEncoded, err := common.EncodeDataAndLimit([]byte("world-ss-tcp"))
	if err != nil {
		t.Fatalf("encode reply payload: %v", err)
	}
	inlet.Input(O2IRecvData{TunnelID: 1, ID: connect.ID, Data: replyEncoded})

	buf := make([]byte, 64)
	if err := ssConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	n, err := ssConn.Read(buf)
	if err != nil {
		t.Fatalf("read shadowsocks reply: %v", err)
	}
	if string(buf[:n]) != "world-ss-tcp" {
		t.Fatalf("reply payload = %q, want %q", string(buf[:n]), "world-ss-tcp")
	}
}

func TestShadowsocksUDPInteroperatesWithGoShadowsocks2(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	listenAddr := "127.0.0.1:" + freeTCPPort(t)
	common := NewSessionCommonInfo(false, ParseEncryptionMethod("None"), nil)
	messages := make(chan ProxyMessage, 8)
	inlet := NewInlet(
		logger,
		1,
		TunnelModeShadowsocks,
		listenAddr,
		"",
		common,
		InletAuthData{Method: DefaultShadowsocksMethod, Password: "secret"},
		func(msg ProxyMessage) { messages <- msg },
		"ss-go-shadowsocks2-udp",
	)
	if err := inlet.Start(); err != nil {
		t.Fatalf("start inlet: %v", err)
	}
	t.Cleanup(func() { _ = inlet.Stop() })

	cipher, err := core.PickCipher(DefaultShadowsocksMethod, nil, "secret")
	if err != nil {
		t.Fatalf("pick cipher: %v", err)
	}
	serverAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		t.Fatalf("resolve server addr: %v", err)
	}
	rawConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen packet: %v", err)
	}
	defer rawConn.Close()
	ssConn := cipher.PacketConn(rawConn)

	targetBytes, err := (TargetAddr{Host: "dns.google", Port: 53}).ToBytes()
	if err != nil {
		t.Fatalf("target to bytes: %v", err)
	}
	if _, err := ssConn.WriteTo(append(targetBytes, []byte("hello-ss-udp")...), serverAddr); err != nil {
		t.Fatalf("write shadowsocks udp payload: %v", err)
	}

	connectMsg := waitProxyMessage(t, messages)
	connect, ok := connectMsg.(I2OConnect)
	if !ok {
		t.Fatalf("expected I2OConnect, got %T", connectMsg)
	}
	if connect.IsTCP {
		t.Fatalf("expected UDP connect, got tcp=true")
	}

	inlet.Input(O2IConnect{TunnelID: 1, ID: connect.ID, Success: true})

	dataMsg := waitProxyMessage(t, messages)
	sendTo, ok := dataMsg.(I2OSendToData)
	if !ok {
		t.Fatalf("expected I2OSendToData, got %T", dataMsg)
	}
	if sendTo.TargetAddr != "dns.google:53" {
		t.Fatalf("target addr = %q, want %q", sendTo.TargetAddr, "dns.google:53")
	}
	decoded, err := common.DecodeData(sendTo.Data)
	if err != nil {
		t.Fatalf("decode udp payload: %v", err)
	}
	if string(decoded) != "hello-ss-udp" {
		t.Fatalf("decoded udp payload = %q, want %q", string(decoded), "hello-ss-udp")
	}

	replyEncoded, err := common.EncodeDataAndLimit([]byte("world-ss-udp"))
	if err != nil {
		t.Fatalf("encode udp reply: %v", err)
	}
	inlet.Input(O2IRecvDataFrom{
		TunnelID:   1,
		ID:         connect.ID,
		RemoteAddr: "1.1.1.1:53",
		Data:       replyEncoded,
	})

	buf := make([]byte, 256)
	if err := rawConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	n, _, err := ssConn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("read shadowsocks udp reply: %v", err)
	}
	target, body, err := parseShadowsocksTarget(buf[:n])
	if err != nil {
		t.Fatalf("parse udp reply target: %v", err)
	}
	if target.String() != "1.1.1.1:53" {
		t.Fatalf("reply target = %q, want %q", target.String(), "1.1.1.1:53")
	}
	if string(body) != "world-ss-udp" {
		t.Fatalf("reply udp payload = %q, want %q", string(body), "world-ss-udp")
	}
}

func waitProxyMessage(t *testing.T, messages <-chan ProxyMessage) ProxyMessage {
	t.Helper()
	select {
	case msg := <-messages:
		return msg
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for proxy message")
		return nil
	}
}
