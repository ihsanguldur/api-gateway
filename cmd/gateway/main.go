package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/ihsanguldur/api-gateway/internal/auth"
	"github.com/ihsanguldur/api-gateway/internal/breaker"
	"github.com/ihsanguldur/api-gateway/internal/config"
	"github.com/ihsanguldur/api-gateway/internal/health"
	"github.com/ihsanguldur/api-gateway/internal/loadbalancer"
	"github.com/ihsanguldur/api-gateway/internal/metrics"
	"github.com/ihsanguldur/api-gateway/internal/proxy"
	"github.com/ihsanguldur/api-gateway/internal/ratelimit"
	"github.com/ihsanguldur/api-gateway/internal/registry"
	"github.com/ihsanguldur/api-gateway/internal/router"
)

func main() {
	configPath := flag.String("config", "config.json", "path to the gateway's JSON config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	reg := registry.New()
	reg.StartSweeper(cfg.BackendTTL, cfg.SweepInterval, nil)

	checker := health.NewChecker(reg)
	checker.Start(cfg.HealthInterval, nil)

	limiter := ratelimit.New(cfg.RateLimit, cfg.RateBurst)
	limiter.StartJanitor(cfg.LimiterSweep, nil)

	keys := auth.NewKeys(cfg.APIKeys...)

	breakers := breaker.NewSet(cfg.BreakerThreshold, cfg.BreakerCooldown)
	breakers.StartJanitor(cfg.BreakerSweep, cfg.BreakerIdle, nil)

	metricsReg := metrics.New()

	mux := http.NewServeMux()
	reg.RegisterRoutes(mux)
	mux.Handle("/metrics", metricsReg.Handler())

	routes := make([]router.Route, len(cfg.Routes))
	for i, r := range cfg.Routes {
		lb, err := loadbalancer.New(r.LB)
		if err != nil {
			log.Fatalf("invalid config: routes[%d]: %v", i, err)
		}
		route := router.Route{Prefix: r.Prefix, Service: r.Service, LB: lb}
		if r.RateLimit > 0 {
			routeLimiter := ratelimit.New(r.RateLimit, r.RateBurst)
			routeLimiter.StartJanitor(cfg.LimiterSweep, nil)
			route.Limiter = routeLimiter
		}
		routes[i] = route
	}
	rt := router.New(routes)
	mux.Handle("/", limiter.Middleware(keys.Middleware(proxy.NewBalancedProxy(rt, reg, breakers, metricsReg, cfg.UpstreamTimeout))))

	log.Printf("gateway listening on %s (config: %s)", cfg.Addr, *configPath)
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil {
		log.Fatal(err)
	}
}
