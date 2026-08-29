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
		// CSP for the embedded SPA (and harmless on JSON). No inline script
		// (Vite output is clean); style-src stays 'self' unless a dependency
		// proves to need inline <style> (see docs/design/04-auth-security.md §8).
		if h.Get("Content-Security-Policy") == "" {
			h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; "+
				"img-src 'self' data:; font-src 'self'; connect-src 'self'; "+
				"frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		}
		next.ServeHTTP(w, r)
	})
}
