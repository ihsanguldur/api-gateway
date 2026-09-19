package loadbalancer

import (
	"sync"

	"github.com/ihsanguldur/api-gateway/internal/registry"
)

type LeastConnections struct {
	mu     sync.Mutex
	active map[string]int
	turn   int
}

func (lc *LeastConnections) Pick(candidates []registry.Backend) (registry.Backend, func(), bool) {
	if len(candidates) == 0 {
		return registry.Backend{}, nil, false
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	if lc.active == nil {
		lc.active = make(map[string]int)
	}

	start := lc.turn % len(candidates)
	lc.turn++

	best := candidates[start]
	for i := 1; i < len(candidates); i++ {
		c := candidates[(start+i)%len(candidates)]
		if lc.active[c.Addr]*best.Weight < lc.active[best.Addr]*c.Weight {
			best = c
		}
	}

	lc.active[best.Addr]++
	addr := best.Addr
	return best, func() { lc.release(addr) }, true
}

func (lc *LeastConnections) release(addr string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.active[addr]--
	if lc.active[addr] == 0 {
		delete(lc.active, addr)
	}
}
