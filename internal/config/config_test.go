package config

import "testing"

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
			"FUND_ENV":      "production",
			"MCP_API_KEY":   strongAdmin,
			"FUND_EDGE_KEY": strongEdge,
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
			"FUND_ENV":      "prod",
			"MCP_API_KEY":   strongAdmin,
			"FUND_EDGE_KEY": strongEdge,
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
