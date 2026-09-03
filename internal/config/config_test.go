package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseUsesSafeDefaults(t *testing.T) {
	cfg, err := Parse(map[string]string{})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.Addr != "127.0.0.1:8765" {
		t.Fatalf("Addr = %q, want 127.0.0.1:8765", cfg.Addr)
	}
	if cfg.ServiceName != "fund-dashboard-go" {
		t.Fatalf("ServiceName = %q, want fund-dashboard-go", cfg.ServiceName)
	}
	if cfg.DBPath != "data/fund.db" {
		t.Fatalf("DBPath = %q, want data/fund.db", cfg.DBPath)
	}
	if cfg.BackupProducerEnabled {
		t.Fatalf("BackupProducerEnabled = true, want false")
	}
	if cfg.AgentOpsEnabled {
		t.Fatalf("AgentOpsEnabled = true, want false by default")
	}
	if cfg.AgentConfirmationSecret != "" {
		t.Fatalf("AgentConfirmationSecret = %q, want empty by default", cfg.AgentConfirmationSecret)
	}
}

func TestParseUsesFundDBPathBeforeLegacyDBPath(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"DB_PATH":      "legacy/fund.db",
		"FUND_DB_PATH": "data/go-fund.db",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.DBPath != "data/go-fund.db" {
		t.Fatalf("DBPath = %q, want FUND_DB_PATH value", cfg.DBPath)
	}
}

func TestParseUsesLegacyDBPathWhenFundDBPathMissing(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"DB_PATH": "legacy/fund.db",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.DBPath != "legacy/fund.db" {
		t.Fatalf("DBPath = %q, want DB_PATH value", cfg.DBPath)
	}
}

func TestParseStaticDirPrefersFundStaticDirThenWebRoot(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"WEB_ROOT":        "/app/web",
		"FUND_STATIC_DIR": "/app/go-web",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.StaticDir != "/app/go-web" {
		t.Fatalf("StaticDir = %q, want FUND_STATIC_DIR value", cfg.StaticDir)
	}

	cfg, err = Parse(map[string]string{
		"WEB_ROOT": "/app/web",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.StaticDir != "/app/web" {
		t.Fatalf("StaticDir = %q, want WEB_ROOT fallback", cfg.StaticDir)
	}
}

func TestParseBackupDirPrefersFundBackupDirThenLegacyBackupDir(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"BACKUP_DIR":      "/app/backups",
		"FUND_BACKUP_DIR": "/app/go-backups",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.BackupDir != "/app/go-backups" {
		t.Fatalf("BackupDir = %q, want FUND_BACKUP_DIR value", cfg.BackupDir)
	}

	cfg, err = Parse(map[string]string{
		"BACKUP_DIR": "/app/backups",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.BackupDir != "/app/backups" {
		t.Fatalf("BackupDir = %q, want BACKUP_DIR fallback", cfg.BackupDir)
	}
}

func TestParseRejectsBackupProducerEnablement(t *testing.T) {
	_, err := Parse(map[string]string{
		"FUND_BACKUP_PRODUCER_ENABLED": "true",
	})
	if err == nil {
		t.Fatalf("Parse returned nil error, want backup producer rejection")
	}
}

func TestParseAgentOpsRequiresExplicitEnablementAndSecret(t *testing.T) {
	_, err := Parse(map[string]string{
		"FUND_AGENT_OPS_ENABLED": "true",
	})
	if err == nil {
		t.Fatalf("Parse returned nil error, want missing confirmation secret rejection")
	}

	cfg, err := Parse(map[string]string{
		"FUND_AGENT_OPS_ENABLED":         "true",
		"FUND_AGENT_CONFIRMATION_SECRET": "test-secret",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !cfg.AgentOpsEnabled {
		t.Fatalf("AgentOpsEnabled = false, want true")
	}
	if cfg.AgentConfirmationSecret != "test-secret" {
		t.Fatalf("AgentConfirmationSecret not preserved for runtime wiring")
	}
}

func TestParseLoadsPublicMCPKey(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"MCP_API_KEY":    "admin-token",
		"PUBLIC_MCP_KEY": "public-token",
		"FUND_EDGE_KEY":  "edge-token",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.AdminKey != "admin-token" {
		t.Fatalf("AdminKey = %q, want admin-token", cfg.AdminKey)
	}
	if cfg.PublicMCPKey != "public-token" {
		t.Fatalf("PublicMCPKey = %q, want public-token", cfg.PublicMCPKey)
	}
	if cfg.EdgeKey != "edge-token" {
		t.Fatalf("EdgeKey = %q, want edge-token", cfg.EdgeKey)
	}
}

func TestParseProductionAllowsEmptyKeysOutsideProd(t *testing.T) {
	for _, envName := range []string{"", "development", "test", "ci", "staging"} {
		t.Run(envName, func(t *testing.T) {
			env := map[string]string{}
			if envName != "" {
				env["FUND_ENV"] = envName
			}
			cfg, err := Parse(env)
			if err != nil {
				t.Fatalf("Parse returned error for FUND_ENV=%q: %v", envName, err)
			}
			if cfg.AdminKey != "" || cfg.EdgeKey != "" {
				t.Fatalf("expected empty keys outside production, got AdminKey=%q EdgeKey=%q", cfg.AdminKey, cfg.EdgeKey)
			}
		})
	}
}

func TestParseProductionRequiresStrongSecrets(t *testing.T) {
	strongAdmin := "prod-admin-key-01"
	strongEdge := "prod-edge-key-0123"
	strongAgent := "prod-agent-secret1"

	t.Run("accepts strong keys", func(t *testing.T) {
		cfg, err := Parse(map[string]string{
			"FUND_ENV":             "production",
			"MCP_API_KEY":          strongAdmin,
			"FUND_EDGE_KEY":        strongEdge,
			"FUND_ALLOWED_ORIGINS": "https://fund.example.com",
		})
		if err != nil {
			t.Fatalf("Parse returned error: %v", err)
		}
		if cfg.Environment != "production" {
			t.Fatalf("Environment = %q, want production", cfg.Environment)
		}
	})

	t.Run("accepts prod alias", func(t *testing.T) {
		_, err := Parse(map[string]string{
			"FUND_ENV":             "prod",
			"MCP_API_KEY":          strongAdmin,
			"FUND_EDGE_KEY":        strongEdge,
			"FUND_ALLOWED_ORIGINS": "https://fund.example.com",
		})
		if err != nil {
			t.Fatalf("Parse returned error for FUND_ENV=prod: %v", err)
		}
	})

	t.Run("rejects empty MCP_API_KEY", func(t *testing.T) {
		_, err := Parse(map[string]string{
			"FUND_ENV":      "production",
			"FUND_EDGE_KEY": strongEdge,
		})
		if err == nil {
			t.Fatalf("Parse returned nil error, want empty MCP_API_KEY rejection")
		}
	})

	t.Run("rejects empty FUND_EDGE_KEY", func(t *testing.T) {
		_, err := Parse(map[string]string{
			"FUND_ENV":    "production",
			"MCP_API_KEY": strongAdmin,
		})
		if err == nil {
			t.Fatalf("Parse returned nil error, want empty FUND_EDGE_KEY rejection")
		}
	})

	t.Run("rejects short secrets", func(t *testing.T) {
		_, err := Parse(map[string]string{
			"FUND_ENV":      "production",
			"MCP_API_KEY":   "too-short",
			"FUND_EDGE_KEY": strongEdge,
		})
		if err == nil {
			t.Fatalf("Parse returned nil error, want short MCP_API_KEY rejection")
		}
	})

	t.Run("rejects placeholders", func(t *testing.T) {
		for _, placeholder := range []string{"change-me", "ci-test-key", "CHANGE-ME", "CI-TEST-KEY"} {
			_, err := Parse(map[string]string{
				"FUND_ENV":      "production",
				"MCP_API_KEY":   placeholder,
				"FUND_EDGE_KEY": strongEdge,
			})
			if err == nil {
				t.Fatalf("Parse returned nil error for placeholder MCP_API_KEY=%q", placeholder)
			}
		}
	})

	t.Run("agent ops requires strong confirmation secret in production", func(t *testing.T) {
		_, err := Parse(map[string]string{
			"FUND_ENV":                       "production",
			"MCP_API_KEY":                    strongAdmin,
			"FUND_EDGE_KEY":                  strongEdge,
			"FUND_AGENT_OPS_ENABLED":         "true",
			"FUND_AGENT_CONFIRMATION_SECRET": "short",
		})
		if err == nil {
			t.Fatalf("Parse returned nil error, want short confirmation secret rejection")
		}

		cfg, err := Parse(map[string]string{
			"FUND_ENV":                       "production",
			"MCP_API_KEY":                    strongAdmin,
			"FUND_EDGE_KEY":                  strongEdge,
			"FUND_AGENT_OPS_ENABLED":         "true",
			"FUND_AGENT_CONFIRMATION_SECRET": strongAgent,
			"FUND_ALLOWED_ORIGINS":           "https://fund.example.com",
		})
		if err != nil {
			t.Fatalf("Parse returned error: %v", err)
		}
		if cfg.AgentConfirmationSecret != strongAgent {
			t.Fatalf("AgentConfirmationSecret = %q, want %q", cfg.AgentConfirmationSecret, strongAgent)
		}
	})
}

func TestRedactedHidesSecretsAndShowsSafeRuntimeFields(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"FUND_HTTP_ADDR":                 "127.0.0.1:9999",
		"MCP_API_KEY":                    "secret-token",
		"PUBLIC_MCP_KEY":                 "public-secret",
		"FUND_EDGE_KEY":                  "edge-secret",
		"DATABASE_URL":                   "file:fund.db",
		"FUND_AGENT_CONFIRMATION_SECRET": "agent-secret",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	redacted := cfg.Redacted()
	if redacted["MCP_API_KEY"] != "[redacted]" {
		t.Fatalf("MCP_API_KEY redaction = %q, want [redacted]", redacted["MCP_API_KEY"])
	}
	if redacted["PUBLIC_MCP_KEY"] != "[redacted]" {
		t.Fatalf("PUBLIC_MCP_KEY redaction = %q, want [redacted]", redacted["PUBLIC_MCP_KEY"])
	}
	if redacted["FUND_EDGE_KEY"] != "[redacted]" {
		t.Fatalf("FUND_EDGE_KEY redaction = %q, want [redacted]", redacted["FUND_EDGE_KEY"])
	}
	if redacted["DATABASE_URL"] != "[redacted]" {
		t.Fatalf("DATABASE_URL redaction = %q, want [redacted]", redacted["DATABASE_URL"])
	}
	if redacted["FUND_DB_PATH"] != "data/fund.db" {
		t.Fatalf("FUND_DB_PATH = %q, want safe default", redacted["FUND_DB_PATH"])
	}
	if redacted["FUND_HTTP_ADDR"] != "127.0.0.1:9999" {
		t.Fatalf("FUND_HTTP_ADDR = %q, want configured addr", redacted["FUND_HTTP_ADDR"])
	}
	if redacted["FUND_STATIC_DIR"] != "" {
		t.Fatalf("FUND_STATIC_DIR = %q, want empty default", redacted["FUND_STATIC_DIR"])
	}
	if redacted["FUND_BACKUP_DIR"] != "" {
		t.Fatalf("FUND_BACKUP_DIR = %q, want empty default", redacted["FUND_BACKUP_DIR"])
	}
	if redacted["FUND_AGENT_CONFIRMATION_SECRET"] != "[redacted]" {
		t.Fatalf("FUND_AGENT_CONFIRMATION_SECRET = %q, want [redacted]", redacted["FUND_AGENT_CONFIRMATION_SECRET"])
	}
}

func TestParseRateLimitEnvs(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"FUND_API_RPM": "900",
		"FUND_MCP_RPM": "30",
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.APIRPM != 900 || cfg.MCPRPM != 30 {
		t.Fatalf("APIRPM=%d MCPRPM=%d, want 900/30", cfg.APIRPM, cfg.MCPRPM)
	}
	// 非法值回退默认。
	cfg, err = Parse(map[string]string{"FUND_API_RPM": "-5", "FUND_MCP_RPM": "abc"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.APIRPM != 600 || cfg.MCPRPM != 120 {
		t.Fatalf("fallback APIRPM=%d MCPRPM=%d, want 600/120", cfg.APIRPM, cfg.MCPRPM)
	}
	// 未设置默认。
	cfg, err = Parse(map[string]string{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.APIRPM != 600 || cfg.MCPRPM != 120 {
		t.Fatalf("defaults APIRPM=%d MCPRPM=%d", cfg.APIRPM, cfg.MCPRPM)
	}
}

func TestParseMCPPreAuthRPM(t *testing.T) {
	cfg, err := Parse(map[string]string{"FUND_MCP_PREAUTH_RPM": "300"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.MCPPreAuthRPM != 300 {
		t.Fatalf("MCPPreAuthRPM = %d, want 300", cfg.MCPPreAuthRPM)
	}
	// 非法值回退默认 600。
	cfg, err = Parse(map[string]string{"FUND_MCP_PREAUTH_RPM": "-1"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.MCPPreAuthRPM != 600 {
		t.Fatalf("fallback MCPPreAuthRPM = %d, want 600", cfg.MCPPreAuthRPM)
	}
	// 未设置默认 600。
	cfg, err = Parse(map[string]string{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.MCPPreAuthRPM != 600 {
		t.Fatalf("default MCPPreAuthRPM = %d, want 600", cfg.MCPPreAuthRPM)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	cfg, err := Parse(map[string]string{
		"FUND_TRUSTED_PROXIES": "10.0.0.0/8, 192.168.1.5, 2001:db8::/32, not-an-ip, ,",
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.TrustedProxies) != 3 {
		t.Fatalf("trusted = %d entries, want 3 (invalid segment + empty dropped)", len(cfg.TrustedProxies))
	}
	if cfg.TrustedProxies[0].String() != "10.0.0.0/8" {
		t.Fatalf("first = %v", cfg.TrustedProxies[0])
	}
	// 裸 IP 归一化 /32。
	if cfg.TrustedProxies[1].String() != "192.168.1.5/32" {
		t.Fatalf("bare IP = %v", cfg.TrustedProxies[1])
	}

	// 未设置 → nil(维持最右一跳行为)。
	cfg, err = Parse(map[string]string{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.TrustedProxies != nil {
		t.Fatalf("unset trusted = %v, want nil", cfg.TrustedProxies)
	}
}

func TestParseBoolEnvRecognizesTruthyAndFalsySpellings(t *testing.T) {
	falsy := []string{"0", "false", "no", "off", "disabled", "FALSE", "Off"}
	for _, raw := range falsy {
		cfg, err := Parse(map[string]string{"FUND_AGENT_OPS_ENABLED": raw})
		if err != nil {
			t.Fatalf("Parse(FUND_AGENT_OPS_ENABLED=%q): %v", raw, err)
		}
		if cfg.AgentOpsEnabled {
			t.Fatalf("FUND_AGENT_OPS_ENABLED=%q parsed as enabled", raw)
		}
	}

	truthy := []string{"1", "true", "yes", "on", "enabled", "TRUE"}
	for _, raw := range truthy {
		cfg, err := Parse(map[string]string{
			"FUND_AGENT_OPS_ENABLED":         raw,
			"FUND_AGENT_CONFIRMATION_SECRET": "agent-confirmation-secret",
		})
		if err != nil {
			t.Fatalf("Parse(FUND_AGENT_OPS_ENABLED=%q): %v", raw, err)
		}
		if !cfg.AgentOpsEnabled {
			t.Fatalf("FUND_AGENT_OPS_ENABLED=%q parsed as disabled", raw)
		}
	}

	// Unknown spellings stay fail-closed (false), not a startup error.
	cfg, err := Parse(map[string]string{"FUND_AGENT_OPS_ENABLED": "maybe"})
	if err != nil {
		t.Fatalf("Parse(unknown bool): %v", err)
	}
	if cfg.AgentOpsEnabled {
		t.Fatal("unknown bool spelling must parse as disabled")
	}
}

func TestParseWarnsOnInvalidEnvValues(t *testing.T) {
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(orig) })

	if _, err := Parse(map[string]string{
		"FUND_API_RPM":           "abc",
		"FUND_MCP_RPM":           "-1",
		"FUND_AUTH_SESSION_TTL":  "not-a-duration",
		"FUND_AGENT_OPS_ENABLED": "maybe",
		"FUND_EDGE_AUTH_ENABLED": "yep",
	}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out := buf.String()
	for _, env := range []string{"FUND_API_RPM", "FUND_MCP_RPM", "FUND_AUTH_SESSION_TTL", "FUND_AGENT_OPS_ENABLED", "FUND_EDGE_AUTH_ENABLED"} {
		if !strings.Contains(out, env) {
			t.Fatalf("startup diagnostics missing env %s\n%s", env, out)
		}
	}

	// Conflicting TTL/MaxAge is lenient at parse time but must be diagnosable.
	buf.Reset()
	if _, err := Parse(map[string]string{
		"FUND_AUTH_SESSION_TTL":     "2160h",
		"FUND_AUTH_SESSION_MAX_AGE": "720h",
	}); err != nil {
		t.Fatalf("Parse(ttl>max_age): %v", err)
	}
	if !strings.Contains(buf.String(), "session TTL exceeds max age") {
		t.Fatalf("missing TTL/MaxAge diagnostic\n%s", buf.String())
	}
}
