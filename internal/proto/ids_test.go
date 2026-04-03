package proto

import "testing"

func TestDecodeUnknownMessageIDReturnsError(t *testing.T) {
	msg, err := Decode(999999, nil)
	if err == nil {
		t.Fatalf("expected unknown message id error, got msg=%v", msg)
	}
}
