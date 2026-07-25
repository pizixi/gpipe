package proto

import (
	"testing"

	"github.com/pizixi/gpipe/internal/pb"
)

func TestDecodeUnknownMessageIDReturnsError(t *testing.T) {
	msg, err := Decode(999999, nil)
	if err == nil {
		t.Fatalf("expected unknown message id error, got msg=%v", msg)
	}
}

func TestLoginAckRuntimeReportSupportRoundTrip(t *testing.T) {
	payload, err := Encode(&pb.LoginAck{
		PlayerID:                    123,
		SupportsTunnelRuntimeReport: true,
	})
	if err != nil {
		t.Fatalf("encode login ack: %v", err)
	}
	msg, err := Decode(MsgServerClientLoginAck, payload)
	if err != nil {
		t.Fatalf("decode login ack: %v", err)
	}
	ack, ok := msg.(*pb.LoginAck)
	if !ok {
		t.Fatalf("decoded message = %T, want *pb.LoginAck", msg)
	}
	if !ack.SupportsTunnelRuntimeReport {
		t.Fatalf("expected runtime report support flag to round trip")
	}
}

func TestTunnelRuntimeReportRoundTrip(t *testing.T) {
	report := &pb.TunnelRuntimeReport{
		TunnelID:  456,
		Component: "inlet",
		Running:   false,
		Error:     "address already in use",
	}
	payload, err := Encode(report)
	if err != nil {
		t.Fatalf("encode runtime report: %v", err)
	}
	msg, err := Decode(MsgClientServerTunnelRuntimeReport, payload)
	if err != nil {
		t.Fatalf("decode runtime report: %v", err)
	}
	got, ok := msg.(*pb.TunnelRuntimeReport)
	if !ok {
		t.Fatalf("decoded message = %T, want *pb.TunnelRuntimeReport", msg)
	}
	if got.TunnelID != report.TunnelID || got.Component != report.Component || got.Running != report.Running || got.Error != report.Error {
		t.Fatalf("decoded report = %+v, want %+v", got, report)
	}
}

func TestClientVersionAndUpgradeMessagesRoundTrip(t *testing.T) {
	login := &pb.LoginReq{Version: "1.2.3", Password: "secret", Platform: "windows-amd64", UpdaterVersion: 1, UpgradeTaskID: "task", UpgradeState: "rolled_back", UpgradeError: "health check failed"}
	payload, err := Encode(login)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(MsgClientServerLoginReq, payload)
	if err != nil {
		t.Fatal(err)
	}
	got := decoded.(*pb.LoginReq)
	if *got != *login {
		t.Fatalf("login = %+v, want %+v", got, login)
	}

	offer := &pb.UpgradeOffer{TaskID: "0123456789abcdef0123456789abcdef", Version: "1.3.0", Platform: "windows-amd64", Size: 123, SHA256: "digest", Signature: "signature", ChunkSize: 131072}
	payload, err = Encode(offer)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = Decode(MsgServerClientUpgradeOfferNtf, payload)
	if err != nil {
		t.Fatal(err)
	}
	gotOffer := decoded.(*pb.UpgradeOffer)
	if *gotOffer != *offer {
		t.Fatalf("offer = %+v, want %+v", gotOffer, offer)
	}
}
