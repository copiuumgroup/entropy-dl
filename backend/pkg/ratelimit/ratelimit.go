package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a token-bucket rate limiter.
type Limiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastTime   time.Time
}

// New creates a new Limiter with the given rate (requests per second) and burst (max tokens).
func New(rate float64, burst int) *Limiter {
	return &Limiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: rate,
		lastTime:   time.Now(),
	}
}

// Allow returns true if a token is available, refilling based on elapsed time.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastTime).Seconds()
	l.tokens += elapsed * l.refillRate
	if l.tokens > l.maxTokens {
		l.tokens = l.maxTokens
	}
	l.lastTime = now

	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}
