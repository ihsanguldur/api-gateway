package breaker

import (
	"log"
	"sync"
	"time"

	"github.com/ihsanguldur/api-gateway/internal/registry"
)

type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	default:
		return "half-open"
	}
}

type Result int

const (
	Success Result = iota
	Failure
	Ignored
)

type entry struct {
	state    State
	failures int
	openedAt time.Time
	probing  bool
	gen      uint64
	lastUsed time.Time
}

type Set struct {
	mu        sync.Mutex
	entries   map[string]*entry
	threshold int
	cooldown  time.Duration
	now       func() time.Time
}

func NewSet(threshold int, cooldown time.Duration) *Set {
	return &Set{
		entries:   make(map[string]*entry),
		threshold: threshold,
		cooldown:  cooldown,
		now:       time.Now,
	}
}

func (s *Set) Filter(candidates []registry.Backend) []registry.Backend {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	out := make([]registry.Backend, 0, len(candidates))
	for _, c := range candidates {
		if s.available(c.Addr, now) {
			out = append(out, c)
		}
	}
	return out
}

func (s *Set) available(addr string, now time.Time) bool {
	e, ok := s.entries[addr]
	if !ok {
		return true
	}
	e.lastUsed = now
	switch e.state {
	case Open:
		return now.Sub(e.openedAt) >= s.cooldown
	case HalfOpen:
		return !e.probing
	default:
		return true
	}
}

func (s *Set) Acquire(addr string) (func(Result), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	e, ok := s.entries[addr]
	if !ok {
		e = &entry{}
		s.entries[addr] = e
	}
	e.lastUsed = now

	if e.state == Open {
		if now.Sub(e.openedAt) < s.cooldown {
			return nil, false
		}
		s.transition(addr, e, HalfOpen, now)
	}
	if e.state == HalfOpen {
		if e.probing {
			return nil, false
		}
		e.probing = true
	}

	gen := e.gen
	return func(r Result) { s.finish(addr, e, gen, r) }, true
}

func (s *Set) finish(addr string, e *entry, gen uint64, r Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e.gen != gen {
		return
	}
	now := s.now()
	switch e.state {
	case Closed:
		switch r {
		case Success:
			e.failures = 0
		case Failure:
			e.failures++
			if e.failures >= s.threshold {
				s.transition(addr, e, Open, now)
			}
		}
	case HalfOpen:
		switch r {
		case Success:
			s.transition(addr, e, Closed, now)
		case Failure:
			s.transition(addr, e, Open, now)
		case Ignored:
			e.probing = false
		}
	}
}

func (s *Set) transition(addr string, e *entry, to State, now time.Time) {
	log.Printf("breaker: %s %s -> %s", addr, e.state, to)
	e.state = to
	e.gen++
	e.probing = false
	e.failures = 0
	if to == Open {
		e.openedAt = now
	}
}

func (s *Set) StartJanitor(interval, idle time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sweepIdle(idle)
			case <-stop:
				return
			}
		}
	}()
}

func (s *Set) sweepIdle(idle time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	for addr, e := range s.entries {
		if now.Sub(e.lastUsed) >= idle {
			delete(s.entries, addr)
		}
	}
}
