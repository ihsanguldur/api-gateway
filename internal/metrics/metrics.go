package metrics

import (
	"cmp"
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"time"
)

type backendStats struct {
	requests     int64
	errors       int64
	totalLatency time.Duration
}

type Registry struct {
	mu    sync.Mutex
	stats map[string]*backendStats
}

func New() *Registry {
	return &Registry{stats: make(map[string]*backendStats)}
}

func (r *Registry) Record(backend string, isError bool, latency time.Duration) {
	if backend == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.stats[backend]
	if !ok {
		s = &backendStats{}
		r.stats[backend] = s
	}
	s.requests++
	if isError {
		s.errors++
	}
	s.totalLatency += latency
}

type Snapshot struct {
	Backend      string  `json:"backend"`
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

func (r *Registry) snapshot() []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]Snapshot, 0, len(r.stats))
	for backend, s := range r.stats {
		var avg float64
		if s.requests > 0 {
			avg = float64(s.totalLatency.Milliseconds()) / float64(s.requests)
		}
		out = append(out, Snapshot{Backend: backend, Requests: s.requests, Errors: s.errors, AvgLatencyMs: avg})
	}
	slices.SortFunc(out, func(a, b Snapshot) int { return cmp.Compare(a.Backend, b.Backend) })
	return out
}

func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(r.snapshot())
	})
}