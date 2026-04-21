package ratelimit

import (
	"sync"
	"time"
)

// bucket is a simple token-bucket counter for one project.
type bucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// Limiter is an in-memory, per-project token bucket rate limiter.
// It is safe for concurrent use. State is lost on restart.
type Limiter struct {
	ratePerSec float64 // max tokens replenished per second
	capacity   float64
	buckets    sync.Map // projectID string -> *bucket
}

// New creates a Limiter allowing up to eventsPerMinute events per project per minute.
func New(eventsPerMinute int) *Limiter {
	if eventsPerMinute <= 0 {
		eventsPerMinute = 1000
	}
	rps := float64(eventsPerMinute) / 60.0
	return &Limiter{
		ratePerSec: rps,
		capacity:   float64(eventsPerMinute),
	}
}

// Allow returns true if the project has remaining quota and consumes one token.
func (l *Limiter) Allow(projectID string) bool {
	v, _ := l.buckets.LoadOrStore(projectID, &bucket{
		tokens: l.capacity,
		last:   time.Now(),
	})
	b := v.(*bucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * l.ratePerSec
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
