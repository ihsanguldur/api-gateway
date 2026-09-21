package ratelimit

import (
	"math"
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

type Limiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
	now     func() time.Time
}

func New(rate float64, burst int) *Limiter {
	return &Limiter{
		rate:    rate,
		burst:   float64(burst),
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
	return false, wait
}

func (l *Limiter) StartJanitor(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.sweepIdle()
			case <-stop:
				return
			}
		}
	}()
}

func (l *Limiter) sweepIdle() {
	l.mu.Lock()
	defer l.mu.Unlock()

	refill := time.Duration(l.burst / l.rate * float64(time.Second))
	now := l.now()
	for key, b := range l.buckets {
		if now.Sub(b.last) >= refill {
			delete(l.buckets, key)
		}
	}
}
