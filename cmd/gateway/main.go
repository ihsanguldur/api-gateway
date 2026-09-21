package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/ihsanguldur/api-gateway/internal/breaker"
	"github.com/ihsanguldur/api-gateway/internal/health"
	"github.com/ihsanguldur/api-gateway/internal/loadbalancer"
	"github.com/ihsanguldur/api-gateway/internal/proxy"
	"github.com/ihsanguldur/api-gateway/internal/ratelimit"
	"github.com/ihsanguldur/api-gateway/internal/registry"
	"github.com/ihsanguldur/api-gateway/internal/router"
)

const (
	backendTTL     = 10 * time.Second
	sweepInternal  = 5 * time.Second
	healthInterval = 2 * time.Second
	rateLimit      = 10
	rateBurst      = 20
	limiterSweep   = 1 * time.Minute

	breakerThreshold = 5
	breakerCooldown  = 10 * time.Second
	breakerSweep     = 1 * time.Minute
	breakerIdle      = 10 * time.Minute
	upstreamTimeout  = 10 * time.Second
)

func main() {
	addr := flag.String("addr", ":8080", "address for the gateway to listen on")
	flag.Parse()

	reg := registry.New()
	reg.StartSweeper(backendTTL, sweepInternal, nil)

	checker := health.NewChecker(reg)
	checker.Start(healthInterval, nil)

	// TODO: hardcoded for now, will be moved to a JSON config file.
	limiter := ratelimit.New(rateLimit, rateBurst)
	limiter.StartJanitor(limiterSweep, nil)

	// TODO: hardcoded for now, will be moved to a JSON config file.
	breakers := breaker.NewSet(breakerThreshold, breakerCooldown)
	breakers.StartJanitor(breakerSweep, breakerIdle, nil)

	mux := http.NewServeMux()
	reg.RegisterRoutes(mux)

	// TODO: hardcoded for now, will be moved to a JSON config file.
	rt := router.New([]router.Route{
		{Prefix: "/api/users", Service: "user-service", LB: &loadbalancer.LeastConnections{}},
		{Prefix: "/api/orders", Service: "order-service", LB: &loadbalancer.WeightedRoundRobin{}},
	})
	mux.Handle("/", limiter.Middleware(proxy.NewBalancedProxy(rt, reg, breakers, upstreamTimeout)))

	log.Printf("gateway listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
