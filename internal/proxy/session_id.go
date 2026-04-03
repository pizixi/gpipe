package proxy

import "sync/atomic"

var sessionCounter atomic.Uint32

// NextSessionID 生成大于 0 的会话编号。
func NextSessionID() uint32 {
	for {
		id := sessionCounter.Add(1)
		if id > 0 {
			return id
		}
	}
}
