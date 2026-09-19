package registry

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type registerRequest struct {
	Addr          string `json:"addr"`
	Service       string `json:"service"`
	Scheme        string `json:"scheme"`
	HealthPath    string `json:"health_path"`
	HealthTimeout string `json:"health_timeout"`
	HealthStatus  int    `json:"health_status"`
	Weight        int    `json:"weight"`
}

type heartbeatRequest struct {
	Addr string `json:"addr"`
}

func (r *Registry) RegisterHandler(w http.ResponseWriter, req *http.Request) {
	var body registerRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Addr == "" || body.Service == "" {
		http.Error(w, "addr and service are required", http.StatusBadRequest)
		return
	}

	if body.Scheme != "" && body.Scheme != "http" && body.Scheme != "https" {
		http.Error(w, "scheme must be http or https", http.StatusBadRequest)
		return
	}

	if body.HealthPath != "" && !strings.HasPrefix(body.HealthPath, "/") {
		http.Error(w, "health_path must start with /", http.StatusBadRequest)
		return
	}

	var timeout time.Duration
	if body.HealthTimeout != "" {
		d, err := time.ParseDuration(body.HealthTimeout)
		if err != nil || d <= 0 {
			http.Error(w, "health_timeout must be a positive duration like 3s", http.StatusBadRequest)
			return
		}
		timeout = d
	}

	if body.HealthStatus != 0 && (body.HealthStatus < 100 || body.HealthStatus > 599) {
		http.Error(w, "health_status must be a valid HTTP status code", http.StatusBadRequest)
		return
	}

	if body.Weight < 0 {
		http.Error(w, "weight must not be negative", http.StatusBadRequest)
		return
	}

	r.Register(Backend{
		Addr:          body.Addr,
		Service:       body.Service,
		Scheme:        body.Scheme,
		HealthPath:    body.HealthPath,
		HealthTimeout: timeout,
		HealthStatus:  body.HealthStatus,
		Weight:        body.Weight,
	})
	w.WriteHeader(http.StatusOK)
}

func (r *Registry) HeartbeatHandler(w http.ResponseWriter, req *http.Request) {
	var body heartbeatRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Addr == "" {
		http.Error(w, "addr is required", http.StatusBadRequest)
		return
	}

	if err := r.Heartbeat(body.Addr); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (r *Registry) ListHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(r.List())
}

func (r *Registry) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/internal/register", r.RegisterHandler)
	mux.HandleFunc("/internal/heartbeat", r.HeartbeatHandler)
	mux.HandleFunc("/internal/backends", r.ListHandler)
}
