package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

func main() {
	addr := flag.String("addr", ":9001", "address to listen on")
	advertise := flag.String("advertise", "", "address to advertise to the gateway (host:port reachable from the gateway); defaults to --addr")
	gateway := flag.String("gateway", "http://localhost:8080", "gateway base URL, for /internal/register and /internal/heartbeat")
	service := flag.String("service", "", "service name to register as (required)")
	name := flag.String("name", "backend", "identifier returned in responses, to tell backends apart")
	heartbeat := flag.Duration("heartbeat", 3*time.Second, "how often to send a heartbeat to the gateway")
	flag.Parse()

	if *service == "" {
		log.Fatal("--service is required")
	}
	adv := *advertise
	if adv == "" {
		adv = *addr
	}

	var mu sync.Mutex
	healthy := true

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ok := healthy
		mu.Unlock()
		if !ok {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/toggle", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		healthy = !healthy
		state := healthy
		mu.Unlock()
		log.Printf("health toggled to %v", state)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"healthy": state})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"backend": *name, "path": r.URL.Path})
	})

	go registerLoop(*gateway, adv, *service, *heartbeat)

	log.Printf("mock backend %q listening on %s (advertising %s as %s)", *name, *addr, adv, *service)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func registerLoop(gateway, addr, service string, interval time.Duration) {
	body, _ := json.Marshal(map[string]string{"addr": addr, "service": service})
	for {
		if err := post(gateway+"/internal/register", body); err != nil {
			log.Printf("register failed, retrying: %v", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	log.Printf("registered with gateway as %s (%s)", addr, service)

	hbBody, _ := json.Marshal(map[string]string{"addr": addr})
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := post(gateway+"/internal/heartbeat", hbBody); err != nil {
			log.Printf("heartbeat failed: %v", err)
		}
	}
}

func post(url string, body []byte) error {
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}