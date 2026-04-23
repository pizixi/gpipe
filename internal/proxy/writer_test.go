package proxy

import (
	"errors"
	"io"
	"net"
	"testing"
)

type testTimeoutError struct{}

func (testTimeoutError) Error() string   { return "i/o timeout" }
func (testTimeoutError) Timeout() bool   { return true }
func (testTimeoutError) Temporary() bool { return false }

func TestIsExpectedNetCloseError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "net closed", err: net.ErrClosed, want: true},
		{name: "eof", err: io.EOF, want: true},
		{name: "unexpected eof", err: io.ErrUnexpectedEOF, want: true},
		{name: "wrapped eof", err: errors.Join(io.EOF, errors.New("peer closed")), want: true},
		{name: "timeout net error", err: testTimeoutError{}, want: true},
		{name: "windows wsarecv", err: errors.New("read tcp 127.0.0.1:1->127.0.0.1:2: wsarecv: A connection attempt failed because the connected party did not properly respond after a period of time"), want: true},
		{name: "broken pipe", err: errors.New("write tcp 127.0.0.1:1->127.0.0.1:2: broken pipe"), want: true},
		{name: "unexpected error", err: errors.New("permission denied"), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isExpectedNetCloseError(tt.err); got != tt.want {
				t.Fatalf("isExpectedNetCloseError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
