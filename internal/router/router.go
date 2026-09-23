package router

import (
	"cmp"
	"slices"
	"strings"

	"github.com/ihsanguldur/api-gateway/internal/loadbalancer"
	"github.com/ihsanguldur/api-gateway/internal/ratelimit"
)

type Route struct {
	Prefix  string
	Service string
	LB      loadbalancer.LoadBalancer
	Limiter *ratelimit.Limiter
}

type Router struct {
	routes []Route
}

func New(routes []Route) *Router {
	sorted := make([]Route, len(routes))
	for i, r := range routes {
		r.Prefix = strings.TrimRight(r.Prefix, "/")
		if r.LB == nil {
			r.LB = &loadbalancer.RoundRobin{}
		}
		sorted[i] = r
	}
	slices.SortStableFunc(sorted, func(a, b Route) int {
		return cmp.Compare(len(b.Prefix), len(a.Prefix))
	})
	return &Router{routes: sorted}
}

func (rt *Router) Match(path string) (Route, bool) {
	for _, r := range rt.routes {
		if path == r.Prefix || strings.HasPrefix(path, r.Prefix+"/") {
			return r, true
		}
	}
	return Route{}, false
}
