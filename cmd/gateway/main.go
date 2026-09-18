package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/ihsanguldur/api-gateway/internal/proxy"
	"github.com/ihsanguldur/api-gateway/internal/registry"
)

const (
	backendTTL    = 10 * time.Second
	sweepInternal = 5 * time.Second
)

func main() {
	backend := flag.String("backend", "http://localhost:9000", "backend address to proxy")
	addr := flag.String("addr", ":8080", "address for the gateway to listen on")
	flag.Parse()

	proxyHandler, err := proxy.NewSingleHostProxy(*backend)
	if err != nil {
		log.Fatalf("invalid backend address: %v", err)
	}

	reg := registry.New()
	reg.StartSweeper(backendTTL, sweepInternal, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/internal/register", reg.RegisterHandler)
	mux.HandleFunc("/internal/heartbeat", reg.HeartbeatHandler)
	mux.HandleFunc("/internal/backends", reg.ListHandler)
	mux.Handle("/", proxyHandler)

	log.Printf("gateway listening on %s, proxying to %s", *addr, *backend)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
