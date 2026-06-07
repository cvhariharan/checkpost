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
	if err == nil || !strings.Contains(err.Error(), "Config.AppConfig.TLSCertPath") {
		t.Fatalf("Load() error = %v, want TLS validation failure", err)
	}
}

func TestConfigValidateOIDC(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "partial config",
			mutate: func(cfg *Config) {
				cfg.OIDCConfig.Issuer = "https://issuer.example.com"
			},
			wantErr: "Config.OIDCConfig.ClientID",
		},
		{
			name: "complete config",
			mutate: func(cfg *Config) {
				cfg.OIDCConfig.Issuer = "https://issuer.example.com"
				cfg.OIDCConfig.ClientID = "checkpost"
				cfg.OIDCConfig.ClientSecret = "secret"
			},
		},
		{
			name: "invalid default role",
			mutate: func(cfg *Config) {
				cfg.OIDCConfig.DefaultRole = "owner"
			},
			wantErr: "Config.OIDCConfig.DefaultRole",
		},
		{
			name: "invalid optional url",
			mutate: func(cfg *Config) {
				cfg.OIDCConfig.AuthURL = "not a url"
			},
			wantErr: "Config.OIDCConfig.AuthURL",
		},
		{
			name: "empty scope",
			mutate: func(cfg *Config) {
				cfg.OIDCConfig.Scopes = []string{"openid", ""}
			},
			wantErr: "Config.OIDCConfig.Scopes",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

func TestConfigValidateResults(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "parquet enabled without root",
			mutate: func(cfg *Config) {
				cfg.ResultsConfig.Parquet.Root = ""
			},
			wantErr: "Config.ResultsConfig.Parquet.Root",
		},
		{
			name: "clickhouse enabled without dsn",
			mutate: func(cfg *Config) {
				cfg.ResultsConfig.ClickHouse.Enabled = true
			},
			wantErr: "Config.ResultsConfig.ClickHouse.DSN",
		},
		{
			name: "clickhouse negative ttl",
			mutate: func(cfg *Config) {
				cfg.ResultsConfig.ClickHouse.TTLDays = -1
			},
			wantErr: "Config.ResultsConfig.ClickHouse.TTLDays",
		},
		{
			name: "ndjson empty path uses stdout",
			mutate: func(cfg *Config) {
				cfg.ResultsConfig.NDJSON.Enabled = true
				cfg.ResultsConfig.NDJSON.Path = ""
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

func TestConfigValidateAlerts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "alerts enabled without smtp host",
			mutate: func(cfg *Config) {
				cfg.AlertsConfig.Enabled = true
			},
		},
		{
			name: "smtp host without port",
			mutate: func(cfg *Config) {
				cfg.AlertsConfig.SMTP.Host = "smtp.example.com"
				cfg.AlertsConfig.SMTP.Port = 0
			},
			wantErr: "Config.AlertsConfig.SMTP.Port",
		},
		{
			name: "smtp host without from address",
			mutate: func(cfg *Config) {
				cfg.AlertsConfig.SMTP.Host = "smtp.example.com"
				cfg.AlertsConfig.SMTP.From = ""
			},
			wantErr: "Config.AlertsConfig.SMTP.From",
		},
		{
			name: "smtp invalid tls mode",
			mutate: func(cfg *Config) {
				cfg.AlertsConfig.SMTP.Host = "smtp.example.com"
				cfg.AlertsConfig.SMTP.TLS = "required"
			},
			wantErr: "Config.AlertsConfig.SMTP.TLS",
		},
		{
			name: "smtp username without password",
			mutate: func(cfg *Config) {
				cfg.AlertsConfig.SMTP.Username = "checkpost"
			},
			wantErr: "Config.AlertsConfig.SMTP.Password",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

func TestConfigValidateOsqueryBootstrapPackages(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pkg     BootstrapPackage
		wantErr string
	}{
		{name: "empty pair"},
		{
			name:    "url only",
			pkg:     BootstrapPackage{URL: "https://packages.example/osquery.deb"},
			wantErr: "Config.AppConfig.OsqueryBootstrap.Linux.DEBAMD64.SHA256",
		},
		{
			name:    "sha only",
			pkg:     BootstrapPackage{SHA256: strings.Repeat("a", 64)},
			wantErr: "Config.AppConfig.OsqueryBootstrap.Linux.DEBAMD64.URL",
		},
		{
			name:    "invalid url",
			pkg:     BootstrapPackage{URL: "not a url", SHA256: strings.Repeat("a", 64)},
			wantErr: "Config.AppConfig.OsqueryBootstrap.Linux.DEBAMD64.URL",
		},
		{
			name:    "invalid sha",
			pkg:     BootstrapPackage{URL: "https://packages.example/osquery.deb", SHA256: "abc"},
			wantErr: "Config.AppConfig.OsqueryBootstrap.Linux.DEBAMD64.SHA256",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.AppConfig.OsqueryBootstrap.Linux.DEBAMD64 = tt.pkg

			err := cfg.Validate()
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

func validConfig() Config {
	return Config{
		AppConfig: AppConfig{
			AdminUsername:        "checkpost_admin",
			AdminPassword:        "checkpost_password",
			RootURL:              "http://localhost:1323",
			SecureCookieKey:      "secure-cookie-key",
			EnrollmentKey:        "enrollment-key",
			PolicyUpdateInterval: time.Hour,
			PolicyStaleAfter:     2 * time.Hour,
		},
		SessionConfig: SessionConfig{
			TTL: 8 * time.Hour,
		},
		DBConfig: DBConfig{
			DBName: "checkpost",
			Host:   "localhost",
			Port:   5432,
			User:   "checkpost",
		},
		ResultsConfig: ResultsConfig{
			Parquet: ParquetConfig{
				Enabled: true,
				Root:    "./data/results",
			},
		},
		AlertsConfig: AlertsConfig{
			SMTP: SMTPConfig{
				Port: 587,
				From: "checkpost@example.com",
				TLS:  "starttls",
			},
		},
	}
}

func assertValidationError(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate() error = %v, want %q", err, want)
	}
}
