package registry

import (
	"encoding/json"
	"net/http"
)

type registerRequest struct {
	Addr    string `json:"addr"`
	Service string `json:"service"`
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
	r.Register(body.Addr, body.Service)
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
