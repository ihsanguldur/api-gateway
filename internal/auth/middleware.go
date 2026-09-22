package auth

import "net/http"

func (k Keys) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !k.valid(r.Header.Get("X-API-Key")) {
			http.Error(w, "invalid or missing API key", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
