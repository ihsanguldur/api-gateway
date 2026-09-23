package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"time"

	"github.com/ihsanguldur/api-gateway/internal/breaker"
	"github.com/ihsanguldur/api-gateway/internal/loadbalancer"
	"github.com/ihsanguldur/api-gateway/internal/metrics"
	"github.com/ihsanguldur/api-gateway/internal/ratelimit"
	"github.com/ihsanguldur/api-gateway/internal/registry"
	"github.com/ihsanguldur/api-gateway/internal/router"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func NewBalancedProxy(rt *router.Router, reg *registry.Registry, breakers *breaker.Set, metricsReg *metrics.Registry, upstreamTimeout time.Duration) http.Handler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = upstreamTimeout

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		var backendAddr string
		defer func() {
			latency := time.Since(start)
			backend := backendAddr
			if backend == "" {
				backend = "-"
			}
			log.Printf("method=%s path=%s backend=%s status=%d latency=%s", req.Method, req.URL.Path, backend, sw.status, latency)
			if backendAddr != "" {
				metricsReg.Record(backendAddr, sw.status >= 500, latency)
			}
		}()

		route, ok := rt.Match(req.URL.Path)
		if !ok {
			http.NotFound(sw, req)
			return
		}

		if route.Limiter != nil {
			if ok, retryAfter := route.Limiter.Allow(ratelimit.ClientIP(req)); !ok {
				ratelimit.Reject(sw, retryAfter)
				return
			}
		}

		candidates := breakers.Filter(reg.HealthyBackends(route.Service))
		b, release, report, ok := pick(route.LB, breakers, candidates)
		if !ok {
			http.Error(sw, "no healthy backend available", http.StatusServiceUnavailable)
			return
		}
		defer release()
		backendAddr = b.Addr

		reported := false
		finish := func(res breaker.Result) {
			if !reported {
				reported = true
				report(res)
			}
		}
		defer finish(breaker.Ignored)

		target := &url.URL{Scheme: b.Scheme, Host: b.Addr}
		rp := httputil.NewSingleHostReverseProxy(target)
		rp.Transport = transport
		rp.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode >= 500 {
				finish(breaker.Failure)
			} else {
				finish(breaker.Success)
			}
			return nil
		}
		rp.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
			if req.Context().Err() != nil {
				finish(breaker.Ignored)
			} else {
				finish(breaker.Failure)
			}
			log.Printf("proxy error: %v", err)
			rw.WriteHeader(http.StatusBadGateway)
		}
		rp.ServeHTTP(sw, req)
	})
}

func pick(lb loadbalancer.LoadBalancer, breakers *breaker.Set, candidates []registry.Backend) (registry.Backend, func(), func(breaker.Result), bool) {
	for {
		b, release, ok := lb.Pick(candidates)
		if !ok {
			return registry.Backend{}, nil, nil, false
		}
		if report, ok := breakers.Acquire(b.Addr); ok {
			return b, release, report, true
		}
		release()
		candidates = slices.DeleteFunc(candidates, func(c registry.Backend) bool { return c.Addr == b.Addr })
	}
}
