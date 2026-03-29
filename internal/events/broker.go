package events

import "sync"

const defaultBufferSize = 32

// EventBroker is a tiny in-process pub/sub helper for buffered channel subscribers.
type EventBroker[T any] struct {
	mu      sync.Mutex
	clients map[chan T]struct{}
	closed  bool
	buffer  int
}

// NewBroker creates a broker with the given default subscription buffer size.
func NewBroker[T any](buffer int) *EventBroker[T] {
	if buffer <= 0 {
		buffer = defaultBufferSize
	}
	return &EventBroker[T]{
		clients: make(map[chan T]struct{}),
		buffer:  buffer,
	}
}

// Subscribe registers a buffered subscriber channel.
func (b *EventBroker[T]) Subscribe() <-chan T {
	return b.SubscribeWithSnapshot(nil, 0)
}

// SubscribeWithSnapshot registers a buffered subscriber after populating an initial snapshot.
// The snapshot callback runs while registration is locked, ensuring a snapshot-first stream.
func (b *EventBroker[T]) SubscribeWithSnapshot(fill func(chan<- T), buffer int) <-chan T {
	if buffer <= 0 {
		buffer = b.buffer
	}
	ch := make(chan T, buffer)

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(ch)
		return ch
	}
	if fill != nil {
		fill(ch)
	}
	b.clients[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *EventBroker[T]) Unsubscribe(ch <-chan T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.clients {
		if c == ch {
			delete(b.clients, c)
			close(c)
			break
		}
	}
}

// Publish fan-outs an event to all subscribers, dropping it for slow consumers.
func (b *EventBroker[T]) Publish(evt T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
			// Slow subscriber: drop event and keep broker non-blocking.
		}
	}
}

// Close closes all subscribers and marks the broker as closed.
func (b *EventBroker[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.clients {
		close(ch)
		delete(b.clients, ch)
	}
}
