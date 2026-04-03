package codec

import (
	"bytes"
	"testing"

	"github.com/pizixi/gpipe/internal/pb"
)

func TestTryExtractFrameReturnsFrameAndRest(t *testing.T) {
	first, err := Encode(-1, &pb.Ping{Ticks: 123})
	if err != nil {
		t.Fatalf("encode first frame: %v", err)
	}
	second, err := Encode(2, &pb.Pong{Ticks: 456})
	if err != nil {
		t.Fatalf("encode second frame: %v", err)
	}

	buffer := append(append([]byte(nil), first...), second...)
	frame, rest, err := TryExtractFrame(buffer, 1024)
	if err != nil {
		t.Fatalf("extract frame: %v", err)
	}
	if !bytes.Equal(rest, second) {
		t.Fatalf("rest mismatch")
	}

	serial, message, err := Decode(frame)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if serial != -1 {
		t.Fatalf("serial = %d, want %d", serial, -1)
	}
	ping, ok := message.(*pb.Ping)
	if !ok {
		t.Fatalf("message = %T, want *pb.Ping", message)
	}
	if ping.Ticks != 123 {
		t.Fatalf("ticks = %d, want %d", ping.Ticks, 123)
	}
}

func TestTryExtractFrameReturnsNilForIncompleteFrame(t *testing.T) {
	frame, err := Encode(-1, &pb.Ping{Ticks: 123})
	if err != nil {
		t.Fatalf("encode frame: %v", err)
	}

	partial := frame[:len(frame)-1]
	extracted, rest, err := TryExtractFrame(partial, 1024)
	if err != nil {
		t.Fatalf("extract partial frame: %v", err)
	}
	if extracted != nil {
		t.Fatalf("expected no extracted frame")
	}
	if !bytes.Equal(rest, partial) {
		t.Fatalf("rest mismatch")
	}
}
