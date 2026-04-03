package proxy

import (
	"fmt"
	"log"
	"runtime/debug"
)

func safeGo(logger *log.Logger, name string, fn func()) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil && logger != nil {
				logger.Printf("%s panic: %v\n%s", name, recovered, debug.Stack())
			}
		}()
		fn()
	}()
}

func safeClose(logger *log.Logger, name string, fn func() error) {
	if fn == nil {
		return
	}
	if err := fn(); err != nil && logger != nil && !isExpectedNetCloseError(err) {
		logger.Printf("%s close error: %v", name, err)
	}
}

func goroutineName(parts ...any) string {
	return fmt.Sprint(parts...)
}
