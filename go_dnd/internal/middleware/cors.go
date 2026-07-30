package middleware

import (
	"net/http"
	"os"
)

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Trace-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "300")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	env := os.Getenv("ENVIRONMENT")
	if env == "" || env == "development" || env == "dev" || env == "local" {
		allowed := []string{
			"http://localhost:4200",
			"http://localhost:4201",
			"http://localhost:4202",
		}
		for _, a := range allowed {
			if origin == a {
				return true
			}
		}
	}
	prod := os.Getenv("ALLOWED_ORIGIN")
	return prod != "" && origin == prod
}
