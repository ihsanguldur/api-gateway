package loadbalancer

import (
	"sync/atomic"

	"github.com/ihsanguldur/api-gateway/internal/registry"
)

type RoundRobin struct {
	next atomic.Uint64
}

func (rr *RoundRobin) Pick(candidates []registry.Backend) (registry.Backend, func(), bool) {
	if len(candidates) == 0 {
		return registry.Backend{}, nil, false
	}
	n := rr.next.Add(1) - 1
	return candidates[n%uint64(len(candidates))], noRelease, true
}
