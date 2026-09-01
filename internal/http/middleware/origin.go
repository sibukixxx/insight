package middleware

import (
	"net/http"
	"net/url"
)

// RestrictOrigin rejects cross-origin browser requests. The server binds to
// localhost by design; without this check, any other local process's web
// page could drive the API using the browser's ambient access to
// localhost.
func RestrictOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !isLoopback(u.Hostname()) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
