package httpapi

import "net/http"

// SecurityHeaders sets defense-in-depth browser hardening headers on the app.
// Edge (reverse proxy) also sets HSTS/CSP for public TLS; this covers direct app access
// (localhost/smoke) and any path that bypasses edge header injection.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Only set if upstream has not already declared (edge may inject stronger CSP).
		if h.Get("X-Content-Type-Options") == "" {
			h.Set("X-Content-Type-Options", "nosniff")
		}
		if h.Get("X-Frame-Options") == "" {
			h.Set("X-Frame-Options", "DENY")
		}
		if h.Get("Referrer-Policy") == "" {
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		}
		if h.Get("Permissions-Policy") == "" {
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		}
		// Reduce MIME sniffing / cross-origin leakage for API JSON responses.
		if h.Get("Cross-Origin-Resource-Policy") == "" {
			h.Set("Cross-Origin-Resource-Policy", "same-site")
		}
		next.ServeHTTP(w, r)
	})
}
