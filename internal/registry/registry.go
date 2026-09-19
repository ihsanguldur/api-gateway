package registry

import (
	"cmp"
	"errors"
	"slices"
	"sync"
	"time"
)

const (
	DefaultScheme        = "http"
	DefaultHealthPath    = "/health"
	DefaultHealthTimeout = 1 * time.Second
	DefaultWeight        = 1
)

type Backend struct {
	Addr          string
	Service       string
	Scheme        string
	HealthPath    string
	HealthTimeout time.Duration
	HealthStatus  int
	Weight        int
	Healthy       bool
	LastSeen      time.Time
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

func (r *Registry) Register(b Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b.Scheme == "" {
		b.Scheme = DefaultScheme
	}
	if b.HealthPath == "" {
		b.HealthPath = DefaultHealthPath
	}
	if b.HealthTimeout <= 0 {
		b.HealthTimeout = DefaultHealthTimeout
	}
	if b.Weight <= 0 {
		b.Weight = DefaultWeight
	}
	b.LastSeen = time.Now()
	b.Healthy = r.backends[b.Addr].Healthy
	r.backends[b.Addr] = b
}

func (r *Registry) SetHealth(addr string, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, ok := r.backends[addr]
	if !ok {
		return
	}
	b.Healthy = healthy
	r.backends[addr] = b
}

func (r *Registry) HealthyBackends(service string) []Backend {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Backend, 0, len(r.backends))
	for _, b := range r.backends {
		if b.Healthy && b.Service == service {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b Backend) int { return cmp.Compare(a.Addr, b.Addr) })
	return out
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
