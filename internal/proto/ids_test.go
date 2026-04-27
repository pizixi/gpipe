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
