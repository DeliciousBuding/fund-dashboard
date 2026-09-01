// Package config parses environment variables with safe defaults, secret redaction,
// and validation (e.g. backup producer is rejected).
package config

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr        = "127.0.0.1:8765"
	defaultDBPath      = "data/fund.db"
	defaultServiceName = "fund-dashboard-go"
	defaultVersion     = "dev"
	// minProductionSecretLen is the floor for production secrets. Generate with:
	// openssl rand -hex 32 (preferred) or openssl rand -hex 16.
	minProductionSecretLen = 16
)

type Config struct {
	Addr                    string
	DBDriver                string // "sqlite" (default) or "pg"
	DBPath                  string // SQLite file path
	PGDSN                   string // PostgreSQL connection string (FUND_PG_DSN)
	StaticDir               string
	BackupDir               string
	ServiceName             string
	Version                 string
	Environment             string
	AdminKey                string // MCP_API_KEY for /api/admin/* and operator MCP auth
	PublicMCPKey            string // PUBLIC_MCP_KEY for read-only public MCP auth
	EdgeKey                 string // FUND_EDGE_KEY injected by the trusted browser edge
	BackupProducerEnabled   bool
	AgentOpsEnabled         bool
	AgentConfirmationSecret string
	// AuthPasswordHash is an argon2id PHC string (FUND_AUTH_PASSWORD_HASH). When
	// set it overrides the DB credential and disables password change (IaC mode).
	AuthPasswordHash  string
	AuthSessionTTL    time.Duration // FUND_AUTH_SESSION_TTL, sliding window (default 720h)
	AuthSessionMaxAge time.Duration // FUND_AUTH_SESSION_MAX_AGE, absolute cap (default 2160h)
	// AuthSecureCookie controls the Secure flag on the session cookie (default
	// true; set FUND_AUTH_SECURE_COOKIE=false only for plain-HTTP LAN access).
	AuthSecureCookie bool
	// EdgeAuthEnabled keeps the legacy X-Fund-Edge-Key browser-write fallback
	// (default true during migration; FUND_EDGE_AUTH_ENABLED=false disables).
	EdgeAuthEnabled bool
	// AllowedOrigins is the browser Origin allowlist for mutations
	// (FUND_ALLOWED_ORIGINS, comma-separated; localhost any port always allowed).
	AllowedOrigins []string
	// APIRPM caps per-IP API requests per minute (FUND_API_RPM, default 600,
	// burst 60) — docs/design/06-security-hardening.md §2.3.
	APIRPM int
	// MCPRPM caps per-key MCP requests per minute (FUND_MCP_RPM, default 120).
	MCPRPM int
	// TrustedProxies is the FUND_TRUSTED_PROXIES CIDR allowlist. When non-empty,
	// X-Forwarded-For is only trusted from direct peers inside these networks
	// (design 06 §2.4). Nil means no allowlist is configured: X-Forwarded-For is
	// untrusted entirely and the request IP fails closed to the direct peer.
	TrustedProxies []*net.IPNet
	raw            map[string]string
}

func Parse(env map[string]string) (Config, error) {
	cfg := Config{
		Addr:              valueOrDefault(env["FUND_HTTP_ADDR"], defaultAddr),
		DBDriver:          strings.ToLower(strings.TrimSpace(env["FUND_DB_DRIVER"])),
		DBPath:            valueOrDefault(valueOrDefault(env["FUND_DB_PATH"], env["DB_PATH"]), defaultDBPath),
		PGDSN:             strings.TrimSpace(env["FUND_PG_DSN"]),
		StaticDir:         valueOrDefault(env["FUND_STATIC_DIR"], env["WEB_ROOT"]),
		BackupDir:         valueOrDefault(env["FUND_BACKUP_DIR"], env["BACKUP_DIR"]),
		ServiceName:       valueOrDefault(env["FUND_SERVICE_NAME"], defaultServiceName),
		Version:           valueOrDefault(env["FUND_VERSION"], defaultVersion),
		Environment:       valueOrDefault(env["FUND_ENV"], "development"),
		AdminKey:          strings.TrimSpace(env["MCP_API_KEY"]),
		PublicMCPKey:      strings.TrimSpace(env["PUBLIC_MCP_KEY"]),
		EdgeKey:           strings.TrimSpace(env["FUND_EDGE_KEY"]),
		AgentOpsEnabled:   parseBoolEnv(env["FUND_AGENT_OPS_ENABLED"], "FUND_AGENT_OPS_ENABLED"),
		AuthPasswordHash:  strings.TrimSpace(env["FUND_AUTH_PASSWORD_HASH"]),
		AuthSessionTTL:    parseDurationEnv(env["FUND_AUTH_SESSION_TTL"], "FUND_AUTH_SESSION_TTL", 720*time.Hour),
		AuthSessionMaxAge: parseDurationEnv(env["FUND_AUTH_SESSION_MAX_AGE"], "FUND_AUTH_SESSION_MAX_AGE", 2160*time.Hour),
		AuthSecureCookie:  parseBoolEnvDefault(env["FUND_AUTH_SECURE_COOKIE"], "FUND_AUTH_SECURE_COOKIE", true),
		EdgeAuthEnabled:   parseBoolEnvDefault(env["FUND_EDGE_AUTH_ENABLED"], "FUND_EDGE_AUTH_ENABLED", true),
		AllowedOrigins:    parseOrigins(env["FUND_ALLOWED_ORIGINS"]),
		APIRPM:            parseRPM(env["FUND_API_RPM"], "FUND_API_RPM", 600),
		MCPRPM:            parseRPM(env["FUND_MCP_RPM"], "FUND_MCP_RPM", 120),
		TrustedProxies:    parseTrustedProxies(env["FUND_TRUSTED_PROXIES"]),
		raw:               copyMap(env),
	}

	if parseBoolEnv(env["FUND_BACKUP_PRODUCER_ENABLED"], "FUND_BACKUP_PRODUCER_ENABLED") {
		return Config{}, errors.New("backup producer is disabled in the Go rewrite")
	}
	if cfg.AuthSessionTTL > cfg.AuthSessionMaxAge {
		slog.Warn("config: session TTL exceeds max age; sliding renewal will be clamped to max age",
			"ttl", cfg.AuthSessionTTL.String(),
			"max_age", cfg.AuthSessionMaxAge.String())
	}
	if cfg.AgentOpsEnabled {
		cfg.AgentConfirmationSecret = strings.TrimSpace(env["FUND_AGENT_CONFIRMATION_SECRET"])
		if cfg.AgentConfirmationSecret == "" {
			return Config{}, errors.New("agent ops requires FUND_AGENT_CONFIRMATION_SECRET")
		}
	}
	if isProductionEnv(cfg.Environment) {
		if err := validateProductionSecrets(cfg); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "production", "prod":
		return true
	default:
		return false
	}
}

func validateProductionSecrets(cfg Config) error {
	if err := requireStrongSecret("MCP_API_KEY", cfg.AdminKey); err != nil {
		return err
	}
	// The edge key is only required while the legacy browser-write fallback is on.
	if cfg.EdgeAuthEnabled {
		if err := requireStrongSecret("FUND_EDGE_KEY", cfg.EdgeKey); err != nil {
			return err
		}
	}
	// PUBLIC_MCP_KEY is optional; when set it is internet-facing on /mcp (auth_request off).
	if cfg.PublicMCPKey != "" {
		if err := requireStrongSecret("PUBLIC_MCP_KEY", cfg.PublicMCPKey); err != nil {
			return err
		}
		if len(cfg.PublicMCPKey) == len(cfg.AdminKey) &&
			subtle.ConstantTimeCompare([]byte(cfg.PublicMCPKey), []byte(cfg.AdminKey)) == 1 {
			return errors.New("production rejects PUBLIC_MCP_KEY equal to MCP_API_KEY")
		}
	}
	if cfg.AgentOpsEnabled {
		if err := requireStrongSecret("FUND_AGENT_CONFIRMATION_SECRET", cfg.AgentConfirmationSecret); err != nil {
			return err
		}
	}
	return nil
}

func requireStrongSecret(name, value string) error {
	if value == "" {
		return fmt.Errorf("production requires non-empty %s", name)
	}
	if len(value) < minProductionSecretLen {
		return fmt.Errorf("production requires %s length >= %d", name, minProductionSecretLen)
	}
	switch strings.ToLower(value) {
	case "change-me", "ci-test-key":
		return fmt.Errorf("production rejects placeholder %s", name)
	}
	return nil
}

func (c Config) Redacted() map[string]string {
	redacted := copyMap(c.raw)
	redacted["FUND_HTTP_ADDR"] = c.Addr
	redacted["FUND_DB_DRIVER"] = c.DBDriver
	redacted["FUND_DB_PATH"] = c.DBPath
	redacted["FUND_PG_DSN"] = c.PGDSN
	redacted["FUND_STATIC_DIR"] = c.StaticDir
	redacted["FUND_BACKUP_DIR"] = c.BackupDir
	redacted["FUND_SERVICE_NAME"] = c.ServiceName
	redacted["FUND_VERSION"] = c.Version
	redacted["FUND_ENV"] = c.Environment
	redacted["FUND_BACKUP_PRODUCER_ENABLED"] = "false"
	redacted["FUND_AGENT_OPS_ENABLED"] = boolString(c.AgentOpsEnabled)
	for key := range redacted {
		if isSecretKey(key) {
			redacted[key] = "[redacted]"
		}
	}
	return redacted
}

func valueOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// parseBoolValue parses both truthy and falsy spellings, reporting whether the
// value was recognized. Empty string is recognized as false; callers handle
// unset-vs-default semantics separately.
func parseBoolValue(value string) (parsed, valid bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled":
		return true, true
	case "", "0", "false", "no", "off", "disabled":
		return false, true
	default:
		return false, false
	}
}

// parseBoolEnv parses a boolean env and warns when a non-empty value is not a
// recognized spelling, so a typo like "ture" is diagnosable instead of
// silently disabling a feature.
func parseBoolEnv(value, name string) bool {
	parsed, valid := parseBoolValue(value)
	if !valid {
		slog.Warn("config: invalid bool env ignored, using false", "env", name, "value", strings.TrimSpace(value))
	}
	return parsed
}

// parseBoolEnvDefault parses a boolean env that keeps the given default when
// unset, warning on invalid non-empty values.
func parseBoolEnvDefault(value, name string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return parseBoolEnv(value, name)
}

// parseDurationEnv parses a Go duration env (e.g. "720h"). Empty or invalid
// values fall back to the default; invalid non-empty values are logged so an
// injected typo is diagnosable at startup instead of silently changing the
// session lifetime.
func parseDurationEnv(value, name string, fallback time.Duration) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil || parsed <= 0 {
		slog.Warn("config: invalid duration env ignored, using default", "env", name, "value", trimmed, "default", fallback.String())
		return fallback
	}
	return parsed
}

// parseRPM parses a non-negative per-minute rate env; empty/invalid values fall
// back to the default and invalid non-empty values are logged.
func parseRPM(value, name string, fallback int) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	n, err := strconv.Atoi(trimmed)
	if err != nil || n <= 0 {
		slog.Warn("config: invalid rate env ignored, using default", "env", name, "value", trimmed, "default", fallback)
		return fallback
	}
	return n
}

// parseTrustedProxies parses a comma-separated CIDR/IP allowlist
// (FUND_TRUSTED_PROXIES). Invalid segments are dropped with a WARN — a segment
// that fails to parse is treated as untrusted (fail-closed), so the resulting
// list only ever shrinks the trusted surface. An unset list (nil) leaves
// X-Forwarded-For untrusted: the effective IP fails closed to the direct peer.
func parseTrustedProxies(value string) []*net.IPNet {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var out []*net.IPNet
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ipnet, err := net.ParseCIDR(trimmed); err == nil {
			out = append(out, ipnet)
			continue
		}
		// Bare IP? Normalize to /32 (or /128 for IPv6) so it participates in the allowlist.
		if ip := net.ParseIP(trimmed); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				_, ipnet, _ := net.ParseCIDR(ip.String() + "/32")
				if ipnet != nil {
					out = append(out, ipnet)
					continue
				}
			}
			_, ipnet, _ := net.ParseCIDR(ip.String() + "/128")
			if ipnet != nil {
				out = append(out, ipnet)
				continue
			}
		}
		slog.Warn("FUND_TRUSTED_PROXIES segment ignored (invalid CIDR/IP)", "value", trimmed)
	}
	return out
}

// parseOrigins splits a comma-separated Origin allowlist. The default covers
// the Vite dev server; localhost on any port is always accepted by the origin
// check itself (see internal/httpapi/origin_check.go).
func parseOrigins(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	}
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func copyMap(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func isSecretKey(key string) bool {
	normalized := strings.ToUpper(key)
	for _, marker := range []string{"SECRET", "TOKEN", "KEY", "PASSWORD", "DATABASE_URL", "DSN"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
