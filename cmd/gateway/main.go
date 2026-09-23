package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"

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

	stopJanitors := make(chan struct{})

	reg := registry.New()
	reg.StartJanitor(cfg.BackendTTL, cfg.SweepInterval, stopJanitors)

	checker := health.NewChecker(reg)
	checker.Start(cfg.HealthInterval, stopJanitors)

	limiter := ratelimit.New(cfg.RateLimit, cfg.RateBurst)
	limiter.StartJanitor(cfg.LimiterSweep, stopJanitors)

	keys := auth.NewKeys(cfg.APIKeys...)

	breakers := breaker.NewSet(cfg.BreakerThreshold, cfg.BreakerCooldown)
	breakers.StartJanitor(cfg.BreakerSweep, cfg.BreakerIdle, stopJanitors)

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
			routeLimiter.StartJanitor(cfg.LimiterSweep, stopJanitors)
			route.Limiter = routeLimiter
		}
		routes[i] = route
	}
	rt := router.New(routes)
	mux.Handle("/", limiter.Middleware(keys.Middleware(proxy.NewBalancedProxy(rt, reg, breakers, metricsReg, cfg.UpstreamTimeout))))

	srv := &http.Server{Addr: cfg.Addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("gateway listening on %s (config: %s)", cfg.Addr, *configPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("gateway crashed: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	log.Printf("shutdown signal received, draining in-flight requests (up to %s)", cfg.ShutdownTimeout)
	close(stopJanitors)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown deadline exceeded, forcing close: %v", err)
	}
	log.Println("gateway stopped")
}