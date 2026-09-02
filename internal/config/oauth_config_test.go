package config

import (
	"strings"
	"testing"
	"time"
)

func TestOAuthDefaults(t *testing.T) {
	cfg, err := Parse(map[string]string{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !cfg.OAuthEnabled {
		t.Fatal("OAuth should be enabled by default so a connector works after upgrade")
	}
	if !cfg.OAuthAutoApprove {
		t.Fatal("auto-approve should default on: logging in is the authorization step")
	}
	if cfg.OAuthAllowWriteScope {
		t.Fatal("fund.write must not be advertised by default")
	}
	if cfg.OAuthAccessTTL != time.Hour {
		t.Fatalf("access TTL = %v, want 1h", cfg.OAuthAccessTTL)
	}
	if cfg.OAuthRefreshTTL != 720*time.Hour {
		t.Fatalf("refresh TTL = %v, want 720h", cfg.OAuthRefreshTTL)
	}
	if cfg.OAuthCodeTTL != time.Minute {
		t.Fatalf("code TTL = %v, want 1m", cfg.OAuthCodeTTL)
	}
	if cfg.OAuthRPM != 60 {
		t.Fatalf("OAuth RPM = %d, want 60", cfg.OAuthRPM)
	}
	if len(cfg.OAuthCIMDHosts) != 0 {
		t.Fatalf("CIMD hosts should be empty until configured (the service applies its own default): %v", cfg.OAuthCIMDHosts)
	}
}

func TestOAuthIssuerResolution(t *testing.T) {
	t.Run("explicit base URL wins and is normalized", func(t *testing.T) {
		cfg, err := Parse(map[string]string{
			"FUND_PUBLIC_BASE_URL": "https://fund.example.com/",
			"FUND_ALLOWED_ORIGINS": "https://other.example.com",
		})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if cfg.OAuthPublicBaseURL != "https://fund.example.com" {
			t.Fatalf("base URL = %q, want the trailing slash trimmed", cfg.OAuthPublicBaseURL)
		}
	})

	t.Run("falls back to the first allowed origin", func(t *testing.T) {
		cfg, err := Parse(map[string]string{
			"FUND_ALLOWED_ORIGINS": "https://fund.example.com, http://localhost:5173",
		})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if cfg.OAuthPublicBaseURL != "https://fund.example.com" {
			t.Fatalf("derived issuer = %q", cfg.OAuthPublicBaseURL)
		}
	})

	t.Run("fallback keeps only scheme and host", func(t *testing.T) {
		cfg, err := Parse(map[string]string{
			"FUND_ALLOWED_ORIGINS": "https://fund.example.com:8443/some/path?x=1#frag",
		})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if cfg.OAuthPublicBaseURL != "https://fund.example.com:8443" {
			t.Fatalf("derived issuer = %q, want scheme://host[:port] only", cfg.OAuthPublicBaseURL)
		}
	})

	t.Run("non-absolute origins are skipped", func(t *testing.T) {
		cfg, err := Parse(map[string]string{
			"FUND_ALLOWED_ORIGINS": "localhost:5173, fund.example.com, https://good.example.com",
		})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if cfg.OAuthPublicBaseURL != "https://good.example.com" {
			t.Fatalf("derived issuer = %q", cfg.OAuthPublicBaseURL)
		}
	})

	t.Run("no fallback when OAuth is disabled", func(t *testing.T) {
		cfg, err := Parse(map[string]string{
			"FUND_OAUTH_ENABLED":   "false",
			"FUND_ALLOWED_ORIGINS": "https://fund.example.com",
		})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if cfg.OAuthEnabled {
			t.Fatal("OAuth should be disabled")
		}
		if cfg.OAuthPublicBaseURL != "" {
			t.Fatalf("issuer should not be derived when OAuth is off: %q", cfg.OAuthPublicBaseURL)
		}
	})
}

func TestProductionRequiresAnHTTPSOAuthIssuer(t *testing.T) {
	base := map[string]string{
		"FUND_ENV":      "production",
		"MCP_API_KEY":   "prod-admin-key-01",
		"FUND_EDGE_KEY": "prod-edge-key-0123",
	}

	// The Vite dev default is http://localhost:5173, which must not become a
	// production issuer.
	if _, err := Parse(base); err == nil {
		t.Fatal("production accepted a deployment with no declared public origin")
	} else if !strings.Contains(err.Error(), "FUND_PUBLIC_BASE_URL") {
		t.Fatalf("error does not name the missing variable: %v", err)
	}

	withHTTP := map[string]string{"FUND_ALLOWED_ORIGINS": "http://fund.example.com"}
	for key, value := range base {
		withHTTP[key] = value
	}
	if _, err := Parse(withHTTP); err == nil {
		t.Fatal("production accepted a plain-http issuer")
	}

	withHTTPS := map[string]string{"FUND_ALLOWED_ORIGINS": "https://fund.example.com"}
	for key, value := range base {
		withHTTPS[key] = value
	}
	cfg, err := Parse(withHTTPS)
	if err != nil {
		t.Fatalf("production rejected an https allowed origin: %v", err)
	}
	if cfg.OAuthPublicBaseURL != "https://fund.example.com" {
		t.Fatalf("issuer = %q", cfg.OAuthPublicBaseURL)
	}

	// Explicitly disabling OAuth removes the requirement.
	disabled := map[string]string{"FUND_OAUTH_ENABLED": "false"}
	for key, value := range base {
		disabled[key] = value
	}
	if _, err := Parse(disabled); err != nil {
		t.Fatalf("production should not require an issuer when OAuth is off: %v", err)
	}
}

func TestOAuthTuningEnvironment(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"FUND_OAUTH_ACCESS_TTL":        "30m",
		"FUND_OAUTH_REFRESH_TTL":       "24h",
		"FUND_OAUTH_CODE_TTL":          "30s",
		"FUND_OAUTH_AUTO_APPROVE":      "false",
		"FUND_OAUTH_ALLOW_WRITE_SCOPE": "true",
		"FUND_OAUTH_RPM":               "30",
		"FUND_OAUTH_CIMD_HOSTS":        "ChatGPT.com, https://apps.example.com/",
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.OAuthAccessTTL != 30*time.Minute || cfg.OAuthRefreshTTL != 24*time.Hour || cfg.OAuthCodeTTL != 30*time.Second {
		t.Fatalf("TTLs not parsed: %v %v %v", cfg.OAuthAccessTTL, cfg.OAuthRefreshTTL, cfg.OAuthCodeTTL)
	}
	if cfg.OAuthAutoApprove {
		t.Fatal("auto-approve should be off")
	}
	if !cfg.OAuthAllowWriteScope {
		t.Fatal("write scope should be on")
	}
	if cfg.OAuthRPM != 30 {
		t.Fatalf("OAuth RPM = %d", cfg.OAuthRPM)
	}
	want := []string{"chatgpt.com", "apps.example.com"}
	if len(cfg.OAuthCIMDHosts) != len(want) {
		t.Fatalf("CIMD hosts = %v, want %v", cfg.OAuthCIMDHosts, want)
	}
	for i := range want {
		if cfg.OAuthCIMDHosts[i] != want[i] {
			t.Fatalf("CIMD host %d = %q, want %q", i, cfg.OAuthCIMDHosts[i], want[i])
		}
	}
}

func TestInvalidOAuthTuningFallsBackToDefaults(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"FUND_OAUTH_ACCESS_TTL": "not-a-duration",
		"FUND_OAUTH_CODE_TTL":   "-5s",
		"FUND_OAUTH_RPM":        "0",
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.OAuthAccessTTL != time.Hour || cfg.OAuthCodeTTL != time.Minute || cfg.OAuthRPM != 60 {
		t.Fatalf("invalid tuning did not fall back: %v %v %d", cfg.OAuthAccessTTL, cfg.OAuthCodeTTL, cfg.OAuthRPM)
	}
}

func TestOAuthSigningKeyIsRedacted(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"FUND_OAUTH_SIGNING_KEY": "-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----",
		"FUND_PUBLIC_BASE_URL":   "https://fund.example.com",
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	redacted := cfg.Redacted()
	if got := redacted["FUND_OAUTH_SIGNING_KEY"]; got != "[redacted]" {
		t.Fatalf("signing key was not redacted: %q", got)
	}
	// The issuer is not a secret and must stay visible for diagnostics.
	if got := redacted["FUND_PUBLIC_BASE_URL"]; got != "https://fund.example.com" {
		t.Fatalf("public base URL was redacted: %q", got)
	}
	if cfg.OAuthSigningKey == "" {
		t.Fatal("signing key was not parsed into the config")
	}
}
