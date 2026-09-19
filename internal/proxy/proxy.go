package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/ihsanguldur/api-gateway/internal/registry"
	"github.com/ihsanguldur/api-gateway/internal/router"
)

func NewBalancedProxy(rt *router.Router, reg *registry.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		route, ok := rt.Match(req.URL.Path)
		if !ok {
			http.NotFound(w, req)
			return
		}

		b, release, ok := route.LB.Pick(reg.HealthyBackends(route.Service))
		if !ok {
			http.Error(w, "no healthy backend available", http.StatusServiceUnavailable)
			return
		}
		defer release()
		target := &url.URL{Scheme: b.Scheme, Host: b.Addr}
		httputil.NewSingleHostReverseProxy(target).ServeHTTP(w, req)
	})
}
