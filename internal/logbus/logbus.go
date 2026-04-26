package logbus

import (
	"sync"
	"time"
)

type Event struct {
	Time   time.Time `json:"time"`
	Level  string    `json:"level"`
	Agent  string    `json:"agent"`
	Msg    string    `json:"msg"`
	Fields any       `json:"fields,omitempty"`
}

// Persister is called for every published event (e.g. to persist to Redis). Non-blocking.
type Persister func(Event)

type Bus struct {
	mu        sync.RWMutex
	subs      map[chan Event]struct{}
	history   []Event
	limit     int
	persister Persister
}

func New(limit int) *Bus {
	return &Bus{
		subs:  make(map[chan Event]struct{}),
		limit: limit,
	}
}

// SetPersister sets a callback invoked for every Publish (e.g. append to Redis). Can be nil.
func (b *Bus) SetPersister(p Persister) {
	b.mu.Lock()
	b.persister = p
	b.mu.Unlock()
}

func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	// history ring
	b.history = append(b.history, e)
	if len(b.history) > b.limit {
		b.history = b.history[len(b.history)-b.limit:]
	}
	persister := b.persister
	// fanout (non-blocking best effort)
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
	b.mu.Unlock()
	if persister != nil {
		persister(e)
	}
}

func (b *Bus) Subscribe(buffer int) (ch chan Event, cancel func()) {
	ch = make(chan Event, buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	cancel = func() {
		b.mu.Lock()
		delete(b.subs, ch)
		close(ch)
		b.mu.Unlock()
	}
	return
}

func (b *Bus) History() []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Event, len(b.history))
	copy(out, b.history)
	return out
}

// Clear removes all buffered history. New subscribers still receive new events.
func (b *Bus) Clear() {
	b.mu.Lock()
	b.history = nil
	b.mu.Unlock()
}
