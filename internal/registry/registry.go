package registry

import (
	"errors"
	"sync"
	"time"
)

type Backend struct {
	Addr     string
	Service  string
	LastSeen time.Time
}

type Registry struct {
	mu       sync.RWMutex
	backends map[string]Backend
}

func New() *Registry {
	return &Registry{
		backends: make(map[string]Backend),
	}
}

func (r *Registry) Register(addr, service string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[addr] = Backend{Addr: addr, Service: service, LastSeen: time.Now()}
}

var ErrNotFound = errors.New("backend not registered")

func (r *Registry) Heartbeat(addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, ok := r.backends[addr]
	if !ok {
		return ErrNotFound
	}
	b.LastSeen = time.Now()
	r.backends[addr] = b
	return nil
}

func (r *Registry) List() []Backend {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Backend, 0, len(r.backends))
	for _, b := range r.backends {
		out = append(out, b)
	}
	return out
}

func (r *Registry) StartSweeper(ttl, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.sweepExpired(ttl)
			case <-stop:
				return
			}
		}
	}()
}

func (r *Registry) sweepExpired(ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for addr, b := range r.backends {
		if now.Sub(b.LastSeen) > ttl {
			delete(r.backends, addr)
		}
	}
}
