package loadbalancer

import "fmt"

func New(name string) (LoadBalancer, error) {
	switch name {
	case "", "round_robin":
		return &RoundRobin{}, nil
	case "least_connections":
		return &LeastConnections{}, nil
	case "weighted_round_robin":
		return &WeightedRoundRobin{}, nil
	default:
		return nil, fmt.Errorf("unknown load balancer %q", name)
	}
}
