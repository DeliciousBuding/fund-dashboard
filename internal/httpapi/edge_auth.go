package httpapi

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

const edgeKeyHeader = "X-Fund-Edge-Key"

// allowedEdgeOrigins lists browser Origins accepted for EdgeAuth mutations.
// Empty Origin is allowed (non-browser clients that present EdgeKey).
// Cross-site browser posts are rejected via Sec-Fetch-Site / Origin mismatch.
var allowedEdgeOrigins = []string{
	"https://fund.vectorcontrol.tech",
	"http://127.0.0.1:5176",
	"http://localhost:5176",
	"http://127.0.0.1:8765",
	"http://localhost:8765",
}

// EdgeAuth protects browser mutations that are authenticated by the reverse
// proxy. The shared key is injected upstream and is never sent to the browser.
//
// Defense-in-depth against CSRF: when a browser sends Sec-Fetch-Site=cross-site
// or a non-allowlisted Origin, the request is rejected even if EdgeKey is valid
// (nginx injects EdgeKey after OIDC, so cookie-authenticated cross-site POSTs
// would otherwise succeed).
func EdgeAuth(edgeKey string) func(http.Handler) http.Handler {
	edgeKey = strings.TrimSpace(edgeKey)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := strings.TrimSpace(r.Header.Get(edgeKeyHeader))
			if edgeKey == "" || len(provided) != len(edgeKey) ||
				subtle.ConstantTimeCompare([]byte(provided), []byte(edgeKey)) != 1 {
				WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			if isUnsafeMethod(r.Method) && !browserMutationAllowed(r) {
				WriteJSON(w, http.StatusForbidden, map[string]any{"error": "origin_not_allowed"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// browserMutationAllowed rejects cross-site browser form/fetch CSRF while
// keeping curl/Hermes-style clients (no Origin / no Sec-Fetch-Site) working
// when they already hold EdgeKey.
func browserMutationAllowed(r *http.Request) bool {
	site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
	switch site {
	case "cross-site":
		return false
	case "same-origin", "same-site", "none":
		// Modern browsers label first-party / user-initiated navigations.
		// Still verify Origin when present.
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Non-browser or same-origin navigations that omit Origin.
		return true
	}
	return originAllowed(origin)
}

func originAllowed(origin string) bool {
	// Normalize trailing slash.
	origin = strings.TrimRight(origin, "/")
	lo := strings.ToLower(origin)
	for _, allowed := range allowedEdgeOrigins {
		if lo == strings.ToLower(allowed) {
			return true
		}
	}
	// Also accept exact parseable http(s) localhost any port for local smoke.
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if (host == "localhost" || host == "127.0.0.1") && (u.Scheme == "http" || u.Scheme == "https") {
		return true
	}
	return false
}
