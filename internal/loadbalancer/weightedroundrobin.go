package loadbalancer

import (
	"sync"

	"github.com/ihsanguldur/api-gateway/internal/registry"
)

type WeightedRoundRobin struct {
	mu      sync.Mutex
	current map[string]int
}

func (w *WeightedRoundRobin) Pick(candidates []registry.Backend) (registry.Backend, func(), bool) {
	if len(candidates) == 0 {
		return registry.Backend{}, nil, false
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	next := make(map[string]int, len(candidates))
	total := 0
	best := 0
	for i, c := range candidates {
		next[c.Addr] = w.current[c.Addr] + c.Weight
		total += c.Weight
		if next[c.Addr] > next[candidates[best].Addr] {
			best = i
		}
	}
	next[candidates[best].Addr] -= total
	w.current = next
	return candidates[best], noRelease, true
}
