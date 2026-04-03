package proxy

import (
	"bytes"
	"testing"
)

func TestProxyMessageQueuePreservesOrderAcrossWrapAround(t *testing.T) {
	q := newProxyMessageQueueWithCapacity(3)

	if !q.Push(I2ODisconnect{ID: 1}) || !q.Push(I2ODisconnect{ID: 2}) || !q.Push(I2ODisconnect{ID: 3}) {
		t.Fatalf("expected initial pushes to succeed")
	}
	if q.Push(I2ODisconnect{ID: 4}) {
		t.Fatalf("expected push on full queue to fail")
	}

	for _, want := range []uint32{1, 2} {
		message, ok := q.Pop()
		if !ok {
			t.Fatalf("expected message %d", want)
		}
		got, ok := message.(I2ODisconnect)
		if !ok || got.ID != want {
			t.Fatalf("pop = %#v, want id=%d", message, want)
		}
	}

	if !q.Push(I2ODisconnect{ID: 4}) || !q.Push(I2ODisconnect{ID: 5}) {
		t.Fatalf("expected wrapped pushes to succeed")
	}

	for _, want := range []uint32{3, 4, 5} {
		message, ok := q.Pop()
		if !ok {
			t.Fatalf("expected message %d", want)
		}
		got, ok := message.(I2ODisconnect)
		if !ok || got.ID != want {
			t.Fatalf("pop = %#v, want id=%d", message, want)
		}
	}
}

func TestByteQueuePreservesOrderAcrossWrapAround(t *testing.T) {
	q := newByteQueueWithCapacity(3)

	payloads := [][]byte{
		[]byte("one"),
		[]byte("two"),
		[]byte("three"),
	}
	for _, payload := range payloads {
		if !q.Push(payload) {
			t.Fatalf("expected push for %q", payload)
		}
	}
	if q.Push([]byte("four")) {
		t.Fatalf("expected push on full queue to fail")
	}

	for _, want := range [][]byte{[]byte("one"), []byte("two")} {
		got, ok := q.Pop()
		if !ok || !bytes.Equal(got, want) {
			t.Fatalf("pop = %q, want %q", got, want)
		}
	}

	if !q.Push([]byte("four")) || !q.Push([]byte("five")) {
		t.Fatalf("expected wrapped pushes to succeed")
	}

	for _, want := range [][]byte{[]byte("three"), []byte("four"), []byte("five")} {
		got, ok := q.Pop()
		if !ok || !bytes.Equal(got, want) {
			t.Fatalf("pop = %q, want %q", got, want)
		}
	}
}
