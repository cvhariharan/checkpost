package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadCreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppConfig.PolicyUpdateInterval != time.Hour {
		t.Fatalf("PolicyUpdateInterval = %v, want %v", cfg.AppConfig.PolicyUpdateInterval, time.Hour)
	}
	if cfg.AppConfig.PolicyStaleAfter != 2*time.Hour {
		t.Fatalf("PolicyStaleAfter = %v, want %v", cfg.AppConfig.PolicyStaleAfter, 2*time.Hour)
	}
	if cfg.AppConfig.SecureCookieKey == "" {
		t.Fatal("SecureCookieKey should not be empty")
	}
	if cfg.AppConfig.EnrollmentKey == "" {
		t.Fatal("EnrollmentKey should not be empty")
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	t.Setenv("CHECKPOST_DB__HOST", "db.internal")
	t.Setenv("CHECKPOST_APP__POLICY_STALE_AFTER", "45m")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DBConfig.Host != "db.internal" {
		t.Fatalf("DBConfig.Host = %q, want %q", cfg.DBConfig.Host, "db.internal")
	}
	if cfg.AppConfig.PolicyStaleAfter != 45*time.Minute {
		t.Fatalf("PolicyStaleAfter = %v, want %v", cfg.AppConfig.PolicyStaleAfter, 45*time.Minute)
	}
}

func TestLoadRejectsInvalidDurations(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[app]\npolicy_update_interval = 'bogus'\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(configPath)
	if err == nil || !strings.Contains(err.Error(), "app.policy_update_interval") {
		t.Fatalf("Load() error = %v, want app.policy_update_interval parse failure", err)
	}
}

func TestLoadRejectsInvalidTLSConfiguration(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	raw := "[app]\nuse_tls = true\nhttp_tls_cert = ''\nhttp_tls_key = ''\n"
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(configPath)
	if err == nil || !strings.Contains(err.Error(), "app.http_tls_cert") {
		t.Fatalf("Load() error = %v, want TLS validation failure", err)
	}
}
