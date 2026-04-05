package client

import (
	"context"
	"testing"
)

func TestRunContextAllowsNilLoggerWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	app := New(Options{
		Server: "tcp://127.0.0.1:8118",
		Key:    "demo",
	})
	if err := app.RunContext(ctx); err != nil {
		t.Fatalf("RunContext() error = %v, want nil", err)
	}
}

func TestValidateStreamDialServerList(t *testing.T) {
	if err := ValidateStreamDialServerList("tcp://127.0.0.1:8118,ws://127.0.0.1:8119"); err != nil {
		t.Fatalf("ValidateStreamDialServerList() error = %v, want nil", err)
	}
	if err := ValidateStreamDialServerList("quic://127.0.0.1:8119"); err == nil {
		t.Fatalf("ValidateStreamDialServerList() error = nil, want non-nil")
	}
}
