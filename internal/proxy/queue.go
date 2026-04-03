package proxy

import "sync"

const (
	defaultProxyQueueCapacity = 1024
	defaultByteQueueCapacity  = 128
)

type proxyMessageQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []ProxyMessage
	closed bool
	max    int
	head   int
	size   int
}

func newProxyMessageQueue() *proxyMessageQueue {
	return newProxyMessageQueueWithCapacity(defaultProxyQueueCapacity)
}

func newProxyMessageQueueWithCapacity(max int) *proxyMessageQueue {
	if max <= 0 {
		max = defaultProxyQueueCapacity
	}
	queue := &proxyMessageQueue{
		max:   max,
		items: make([]ProxyMessage, max),
	}
	queue.cond = sync.NewCond(&queue.mu)
	return queue
}

func (q *proxyMessageQueue) Push(message ProxyMessage) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	if q.size >= q.max {
		return false
	}
	tail := (q.head + q.size) % q.max
	q.items[tail] = message
	q.size++
	q.cond.Signal()
	return true
}

func (q *proxyMessageQueue) Pop() (ProxyMessage, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.size == 0 && !q.closed {
		q.cond.Wait()
	}
	if q.size == 0 {
		return nil, false
	}
	message := q.items[q.head]
	q.items[q.head] = nil
	q.head = (q.head + 1) % q.max
	q.size--
	return message, true
}

func (q *proxyMessageQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	for i := range q.items {
		q.items[i] = nil
	}
	q.head = 0
	q.size = 0
	q.cond.Broadcast()
}

type byteQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  [][]byte
	closed bool
	max    int
	head   int
	size   int
}

func newByteQueue() *byteQueue {
	return newByteQueueWithCapacity(defaultByteQueueCapacity)
}

func newByteQueueWithCapacity(max int) *byteQueue {
	if max <= 0 {
		max = defaultByteQueueCapacity
	}
	queue := &byteQueue{
		max:   max,
		items: make([][]byte, max),
	}
	queue.cond = sync.NewCond(&queue.mu)
	return queue
}

func (q *byteQueue) Push(message []byte) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	if q.size >= q.max {
		return false
	}
	tail := (q.head + q.size) % q.max
	q.items[tail] = message
	q.size++
	q.cond.Signal()
	return true
}

func (q *byteQueue) Pop() ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.size == 0 && !q.closed {
		q.cond.Wait()
	}
	if q.size == 0 {
		return nil, false
	}
	message := q.items[q.head]
	q.items[q.head] = nil
	q.head = (q.head + 1) % q.max
	q.size--
	return message, true
}

func (q *byteQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	for i := range q.items {
		q.items[i] = nil
	}
	q.head = 0
	q.size = 0
	q.cond.Broadcast()
}
