package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

const edgeKeyHeader = "X-Fund-Edge-Key"

// Origin allowlist for browser mutations is configured via FUND_ALLOWED_ORIGINS
// (config.Config.AllowedOrigins); exact-scheme+host+port match. Any
// http(s)://localhost / 127.0.0.1 origin on any port is always accepted for
// local development and smoke tests.

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// browserMutationAllowed rejects cross-site browser form/fetch CSRF while
// keeping curl/ops-style clients (no Origin / no Sec-Fetch-Site) working when
// they already authenticated.
func browserMutationAllowed(r *http.Request, allowedOrigins []string) bool {
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
	return originAllowed(origin, allowedOrigins)
}

func originAllowed(origin string, allowedOrigins []string) bool {
	// Normalize trailing slash.
	origin = strings.TrimRight(origin, "/")
	lo := strings.ToLower(origin)
	for _, allowed := range allowedOrigins {
		if lo == strings.ToLower(strings.TrimSpace(allowed)) {
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
