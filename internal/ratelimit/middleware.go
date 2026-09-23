package ratelimit

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"time"
)

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, retryAfter := l.Allow(ClientIP(r))
		if !ok {
			Reject(w, retryAfter)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Reject(w http.ResponseWriter, retryAfter time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
	http.Error(w, "too many requests", http.StatusTooManyRequests)
}

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
