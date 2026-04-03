package proxy

import (
	"testing"
	"time"
)

func TestFlowControllerCloseUnblocksAcquire(t *testing.T) {
	flow := NewFlowController(8)
	if err := flow.Acquire(8); err != nil {
		t.Fatalf("initial acquire failed: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- flow.Acquire(8)
	}()

	time.Sleep(50 * time.Millisecond)
	flow.Close()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected blocked acquire to fail after close")
		}
	case <-time.After(time.Second):
		t.Fatalf("flow close did not unblock acquire")
	}
}
