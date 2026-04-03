package proxy

import (
	"io"
	"log"
	"net"
	"testing"
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
