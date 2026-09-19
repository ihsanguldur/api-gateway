package loadbalancer

import "github.com/ihsanguldur/api-gateway/internal/registry"

type LoadBalancer interface {
	Pick(candidates []registry.Backend) (backend registry.Backend, release func(), ok bool)
}

func noRelease() {}
