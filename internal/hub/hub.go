package hub

import "sync"

// Hub broadcasts a notification to all subscribed SSE clients.
type Hub struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

// New returns an initialized Hub.
func New() *Hub {
	return &Hub{clients: make(map[chan struct{}]struct{})}
}

// Subscribe registers a new listener and returns the notification channel and
// an unsubscribe function that must be called when the client disconnects.
func (h *Hub) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.clients, ch)
		close(ch)
		h.mu.Unlock()
	}
}

// Notify sends a signal to every subscribed client. Slow clients that have
// not consumed the previous signal are skipped — they will catch the next one.
func (h *Hub) Notify() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
