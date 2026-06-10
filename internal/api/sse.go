package api

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// SSEEvent is one Server-Sent Event payload.
type SSEEvent struct {
	Event string
	Data  interface{}
}

// Broadcaster multiplexes events to all subscribed clients.
type Broadcaster struct {
	mu   sync.RWMutex
	subs map[chan SSEEvent]struct{}
}

// NewBroadcaster returns an empty broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: map[chan SSEEvent]struct{}{}}
}

// Subscribe returns a channel of events and an unsubscribe function.
func (b *Broadcaster) Subscribe(buffer int) (<-chan SSEEvent, func()) {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan SSEEvent, buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
}

// Publish non-blockingly sends the event to all subscribers. Slow
// subscribers get their messages dropped to keep the broadcaster fast.
func (b *Broadcaster) Publish(ev SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// drop
		}
	}
}

// HandleSSE is a gin handler that keeps a stream open until the client
// disconnects. The provided event channel must come from Subscribe.
func HandleSSE(c *gin.Context, ch <-chan SSEEvent) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(200)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			c.SSEvent(ev.Event, ev.Data)
			flusher.Flush()
		}
	}
}
