package httpapi

import "net/http"

// baseContentSecurityPolicy is the strict CSP applied to every surface except the
// browser-facing OAuth consent page. The consent page (GET /oauth/authorize)
// extends form-action with the validated client redirect origin, because Chrome
// enforces form-action against every hop of a form-submission redirect chain.
// form-action must remain the LAST directive so that extension is a pure append.
const baseContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; font-src 'self'; connect-src 'self'; " +
	"frame-ancestors 'none'; base-uri 'none'; form-action 'self'"

// oauthBrowserPath reports whether a path is a browser-facing surface that a
// remote MCP client (ChatGPT/Claude/Cursor) opens in a popup. A popup document
// served with Cross-Origin-Opener-Policy: same-origin is severed from its
// cross-origin opener, so the client observes popup.closed == true while the
// consent/login page is still on screen and aborts the OAuth flow. These paths
// opt out of COOP isolation (unsafe-none); they carry no cross-origin secret
// data, and every other surface keeps same-origin isolation.
func oauthBrowserPath(p string) bool {
	switch p {
	case "/oauth/authorize", "/oauth/consent", "/login":
		return true
	default:
		return false
	}
}

// SecurityHeaders sets defense-in-depth browser hardening headers on the app.
// Edge (reverse proxy) also sets HSTS/CSP for public TLS; this covers direct app access
// (localhost/smoke) and any path that bypasses edge header injection.
//
// secure mirrors cfg.AuthSecureCookie (FUND_AUTH_SECURE_COOKIE) — HSTS is only
// sent when the session cookie is Secure, since HSTS on a plain-HTTP deployment
// would hard-brick browser access until the domain migrates to TLS (design
// docs/design/06-security-hardening.md §2.1).
func SecurityHeaders(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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
			// COOP isolates the top-level window from cross-origin popups/windows —
			// mitigates speculative-execution / window-name side channels for a
			// session-authenticated single-tenant app (design 06 §2.1). The OAuth
			// popup surfaces are the one deliberate exception: same-origin there
			// severs the popup from the ChatGPT/Claude opener and breaks login.
			if h.Get("Cross-Origin-Opener-Policy") == "" {
				if oauthBrowserPath(r.URL.Path) {
					h.Set("Cross-Origin-Opener-Policy", "unsafe-none")
				} else {
					h.Set("Cross-Origin-Opener-Policy", "same-origin")
				}
			}
			// CSP for the embedded SPA (and harmless on JSON). No inline script
			// (Vite output is clean); style-src stays 'self' unless a dependency
			// proves to need inline <style> (see docs/design/04-auth-security.md §8).
			if h.Get("Content-Security-Policy") == "" {
				h.Set("Content-Security-Policy", baseContentSecurityPolicy)
			}
			if secure && h.Get("Strict-Transport-Security") == "" {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
