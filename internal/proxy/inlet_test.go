package proxy

import (
	"errors"
	"io"
	"log"
	"net"
	"testing"
	"time"
)

func TestInletSessionLimitReleasesCapacityAfterClose(t *testing.T) {
	inlet := newLifecycleTestInlet()
	inlet.slots = make(chan struct{}, 1)
	peer := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}

	first, err := inlet.runSession(1, &testPeerWriter{}, peer, func() ContextHandler {
		return NewUniversalContext()
	}, nil, "", false)
	if err != nil {
		t.Fatalf("start first session: %v", err)
	}
	if len(inlet.slots) != 1 {
		t.Fatalf("reserved slots = %d, want 1", len(inlet.slots))
	}

	if _, err := inlet.runSession(2, &testPeerWriter{}, peer, func() ContextHandler {
		return NewUniversalContext()
	}, nil, "", false); !errors.Is(err, errInletSessionLimit) {
		t.Fatalf("second session error = %v, want session limit", err)
	}

	inlet.closeSession(first.id)
	if len(inlet.slots) != 0 {
		t.Fatalf("reserved slots after close = %d, want 0", len(inlet.slots))
	}

	third, err := inlet.runSession(3, &testPeerWriter{}, peer, func() ContextHandler {
		return NewUniversalContext()
	}, nil, "", false)
	if err != nil {
		t.Fatalf("start session after capacity release: %v", err)
	}
	inlet.closeSession(third.id)
	if err := inlet.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestInletStopRejectsNewAsyncWorkAndSessions(t *testing.T) {
	inlet := newLifecycleTestInlet()
	if err := inlet.Stop(); err != nil {
		t.Fatal(err)
	}

	ran := make(chan struct{}, 1)
	if inlet.runAsync("after-stop", func() { ran <- struct{}{} }) {
		t.Fatal("runAsync accepted work after Stop")
	}
	select {
	case <-ran:
		t.Fatal("work ran after Stop")
	default:
	}

	peer := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	if _, err := inlet.runSession(1, &testPeerWriter{}, peer, func() ContextHandler {
		return NewUniversalContext()
	}, nil, "", false); !errors.Is(err, errInletStopped) {
		t.Fatalf("session error = %v, want inlet stopped", err)
	}
	if len(inlet.slots) != 0 {
		t.Fatalf("reserved slots after rejected session = %d, want 0", len(inlet.slots))
	}
}

func TestNextTemporaryAcceptDelayUsesBoundedExponentialBackoff(t *testing.T) {
	delay := time.Duration(0)
	for _, want := range []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond} {
		delay = nextTemporaryAcceptDelay(delay)
		if delay != want {
			t.Fatalf("delay = %s, want %s", delay, want)
		}
	}
	if got := nextTemporaryAcceptDelay(maxTemporaryAcceptDelay); got != maxTemporaryAcceptDelay {
		t.Fatalf("capped delay = %s, want %s", got, maxTemporaryAcceptDelay)
	}
}

func newLifecycleTestInlet() *Inlet {
	return NewInlet(
		log.New(io.Discard, "", 0),
		1,
		TunnelModeTCP,
		"127.0.0.1:0",
		"127.0.0.1:9",
		NewSessionCommonInfo(false, EncryptionNone, nil),
		InletAuthData{},
		func(ProxyMessage) {},
		"lifecycle-test",
	)
}
