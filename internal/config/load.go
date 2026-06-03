package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const envPrefix = "CHECKPOST_"

var (
	defaultPolicyUpdateInterval = time.Hour
	defaultPolicyStaleAfter     = 2 * time.Hour
	defaultSessionTTL           = 8 * time.Hour
	configValidator             = validator.New()
)

type rawConfig struct {
	AppConfig              rawAppConfig              `koanf:",squash"`
	OIDCConfig             rawOIDCConfig             `koanf:",squash"`
	SessionConfig          rawSessionConfig          `koanf:",squash"`
	DBConfig               rawDBConfig               `koanf:",squash"`
	ResultsConfig          rawResultsConfig          `koanf:",squash"`
	OsqueryBootstrapConfig rawOsqueryBootstrapConfig `koanf:",squash"`
	AlertsConfig           rawAlertsConfig           `koanf:",squash"`
}

type rawAlertsConfig struct {
	Enabled      bool   `koanf:"alerts.enabled"`
	SMTPHost     string `koanf:"alerts.smtp.host"`
	SMTPPort     int    `koanf:"alerts.smtp.port"`
	SMTPUsername string `koanf:"alerts.smtp.username"`
	SMTPPassword string `koanf:"alerts.smtp.password"`
	SMTPFrom     string `koanf:"alerts.smtp.from"`
	SMTPTLS      string `koanf:"alerts.smtp.tls"`
}

type rawAppConfig struct {
	AdminUsername        string `koanf:"app.admin_username"`
	AdminPassword        string `koanf:"app.admin_password"`
	TLSCertPath          string `koanf:"app.http_tls_cert"`
	TLSKeyPath           string `koanf:"app.http_tls_key"`
	RootURL              string `koanf:"app.root_url"`
	UseTLS               bool   `koanf:"app.use_tls"`
	SecureCookieKey      string `koanf:"app.secure_cookie_key"`
	EnrollmentKey        string `koanf:"app.enrollment_key"`
	PolicyUpdateInterval string `koanf:"app.policy_update_interval"`
	PolicyStaleAfter     string `koanf:"app.policy_stale_after"`
}

type rawOIDCConfig struct {
	Issuer          string   `koanf:"app.oidc.issuer"`
	ClientID        string   `koanf:"app.oidc.client_id"`
	ClientSecret    string   `koanf:"app.oidc.client_secret"`
	Label           string   `koanf:"app.oidc.label"`
	RedirectURL     string   `koanf:"app.oidc.redirect_url"`
	AuthURL         string   `koanf:"app.oidc.auth_url"`
	TokenURL        string   `koanf:"app.oidc.token_url"`
	Scopes          []string `koanf:"app.oidc.scopes"`
	GroupsClaim     string   `koanf:"app.oidc.groups_claim"`
	AllowedDomains  []string `koanf:"app.oidc.allowed_domains"`
	AutoCreateUsers bool     `koanf:"app.oidc.auto_create_users"`
	DefaultRole     string   `koanf:"app.oidc.default_role"`
}

type rawSessionConfig struct {
	TTL string `koanf:"app.session.ttl"`
}

type rawDBConfig struct {
	DBName   string `koanf:"db.dbname"`
	Host     string `koanf:"db.host"`
	Port     int    `koanf:"db.port"`
	User     string `koanf:"db.user"`
	Password string `koanf:"db.password"`
}

type rawResultsConfig struct {
	ParquetEnabled    bool   `koanf:"results.parquet.enabled"`
	ParquetRoot       string `koanf:"results.parquet.root"`
	ParquetDuckDBPath string `koanf:"results.parquet.duckdb_path"`

	NDJSONEnabled  bool   `koanf:"results.ndjson.enabled"`
	NDJSONPath     string `koanf:"results.ndjson.path"`
	NDJSONRequired bool   `koanf:"results.ndjson.required"`

	ClickHouseEnabled  bool   `koanf:"results.clickhouse.enabled"`
	ClickHouseDSN      string `koanf:"results.clickhouse.dsn"`
	ClickHouseTable    string `koanf:"results.clickhouse.table"`
	ClickHouseTTLDays  int    `koanf:"results.clickhouse.ttl_days"`
	ClickHouseRequired bool   `koanf:"results.clickhouse.required"`
}

type rawOsqueryBootstrapConfig struct {
	Enabled                 bool   `koanf:"osquery_bootstrap.enabled"`
	LinuxDEBAMD64URL        string `koanf:"osquery_bootstrap.linux.deb_amd64.url"`
	LinuxDEBAMD64SHA256     string `koanf:"osquery_bootstrap.linux.deb_amd64.sha256"`
	LinuxDEBARM64URL        string `koanf:"osquery_bootstrap.linux.deb_arm64.url"`
	LinuxDEBARM64SHA256     string `koanf:"osquery_bootstrap.linux.deb_arm64.sha256"`
	LinuxRPMAMD64URL        string `koanf:"osquery_bootstrap.linux.rpm_amd64.url"`
	LinuxRPMAMD64SHA256     string `koanf:"osquery_bootstrap.linux.rpm_amd64.sha256"`
	LinuxRPMARM64URL        string `koanf:"osquery_bootstrap.linux.rpm_arm64.url"`
	LinuxRPMARM64SHA256     string `koanf:"osquery_bootstrap.linux.rpm_arm64.sha256"`
	MacOSPKGUniversalURL    string `koanf:"osquery_bootstrap.macos.pkg_universal.url"`
	MacOSPKGUniversalSHA256 string `koanf:"osquery_bootstrap.macos.pkg_universal.sha256"`
	WindowsMSIAMD64URL      string `koanf:"osquery_bootstrap.windows.msi_amd64.url"`
	WindowsMSIAMD64SHA256   string `koanf:"osquery_bootstrap.windows.msi_amd64.sha256"`
}

func Load(configFile string) (Config, error) {
	k := koanf.New(".")

	if err := loadDefaults(k); err != nil {
		return Config{}, fmt.Errorf("load default config: %w", err)
	}
	if err := loadOrCreateConfigFile(k, configFile); err != nil {
		return Config{}, fmt.Errorf("load config file: %w", err)
	}
	if err := loadEnvConfig(k); err != nil {
		return Config{}, fmt.Errorf("load environment config: %w", err)
	}

	var raw rawConfig
	if err := k.UnmarshalWithConf("", &raw, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg, err := raw.toConfig()
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (r rawConfig) toConfig() (Config, error) {
	policyUpdateInterval, err := parsePositiveDuration("app.policy_update_interval", r.AppConfig.PolicyUpdateInterval, defaultPolicyUpdateInterval)
	if err != nil {
		return Config{}, err
	}
	policyStaleAfter, err := parsePositiveDuration("app.policy_stale_after", r.AppConfig.PolicyStaleAfter, defaultPolicyStaleAfter)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := parsePositiveDuration("app.session.ttl", r.SessionConfig.TTL, defaultSessionTTL)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppConfig: AppConfig{
			AdminUsername:        r.AppConfig.AdminUsername,
			AdminPassword:        r.AppConfig.AdminPassword,
			TLSCertPath:          r.AppConfig.TLSCertPath,
			TLSKeyPath:           r.AppConfig.TLSKeyPath,
			RootURL:              r.AppConfig.RootURL,
			UseTLS:               r.AppConfig.UseTLS,
			SecureCookieKey:      r.AppConfig.SecureCookieKey,
			EnrollmentKey:        r.AppConfig.EnrollmentKey,
			PolicyUpdateInterval: policyUpdateInterval,
			PolicyStaleAfter:     policyStaleAfter,
			OsqueryBootstrap: OsqueryBootstrapConfig{
				Enabled: r.OsqueryBootstrapConfig.Enabled,
				Linux: LinuxBootstrapConfig{
					DEBAMD64: packageConfig(r.OsqueryBootstrapConfig.LinuxDEBAMD64URL, r.OsqueryBootstrapConfig.LinuxDEBAMD64SHA256),
					DEBARM64: packageConfig(r.OsqueryBootstrapConfig.LinuxDEBARM64URL, r.OsqueryBootstrapConfig.LinuxDEBARM64SHA256),
					RPMAMD64: packageConfig(r.OsqueryBootstrapConfig.LinuxRPMAMD64URL, r.OsqueryBootstrapConfig.LinuxRPMAMD64SHA256),
					RPMARM64: packageConfig(r.OsqueryBootstrapConfig.LinuxRPMARM64URL, r.OsqueryBootstrapConfig.LinuxRPMARM64SHA256),
				},
				MacOS: MacOSBootstrapConfig{
					PKGUniversal: packageConfig(r.OsqueryBootstrapConfig.MacOSPKGUniversalURL, r.OsqueryBootstrapConfig.MacOSPKGUniversalSHA256),
				},
				Windows: WindowsBootstrapConfig{
					MSIAMD64: packageConfig(r.OsqueryBootstrapConfig.WindowsMSIAMD64URL, r.OsqueryBootstrapConfig.WindowsMSIAMD64SHA256),
				},
			},
		},
		OIDCConfig: OIDCConfig{
			Issuer:          strings.TrimSpace(r.OIDCConfig.Issuer),
			ClientID:        strings.TrimSpace(r.OIDCConfig.ClientID),
			ClientSecret:    strings.TrimSpace(r.OIDCConfig.ClientSecret),
			Label:           r.OIDCConfig.Label,
			RedirectURL:     strings.TrimSpace(r.OIDCConfig.RedirectURL),
			AuthURL:         strings.TrimSpace(r.OIDCConfig.AuthURL),
			TokenURL:        strings.TrimSpace(r.OIDCConfig.TokenURL),
			Scopes:          r.OIDCConfig.Scopes,
			GroupsClaim:     r.OIDCConfig.GroupsClaim,
			AllowedDomains:  r.OIDCConfig.AllowedDomains,
			AutoCreateUsers: r.OIDCConfig.AutoCreateUsers,
			DefaultRole:     strings.TrimSpace(r.OIDCConfig.DefaultRole),
		},
		SessionConfig: SessionConfig{
			TTL: sessionTTL,
		},
		DBConfig: DBConfig{
			DBName:   r.DBConfig.DBName,
			Host:     r.DBConfig.Host,
			Port:     r.DBConfig.Port,
			User:     r.DBConfig.User,
			Password: r.DBConfig.Password,
		},
		ResultsConfig: ResultsConfig{
			Parquet: ParquetConfig{
				Enabled:    r.ResultsConfig.ParquetEnabled,
				Root:       r.ResultsConfig.ParquetRoot,
				DuckDBPath: r.ResultsConfig.ParquetDuckDBPath,
			},
			NDJSON: NDJSONConfig{
				Enabled:  r.ResultsConfig.NDJSONEnabled,
				Path:     r.ResultsConfig.NDJSONPath,
				Required: r.ResultsConfig.NDJSONRequired,
			},
			ClickHouse: ClickHouseConfig{
				Enabled:  r.ResultsConfig.ClickHouseEnabled,
				DSN:      strings.TrimSpace(r.ResultsConfig.ClickHouseDSN),
				Table:    strings.TrimSpace(r.ResultsConfig.ClickHouseTable),
				TTLDays:  r.ResultsConfig.ClickHouseTTLDays,
				Required: r.ResultsConfig.ClickHouseRequired,
			},
		},
		AlertsConfig: AlertsConfig{
			Enabled: r.AlertsConfig.Enabled,
			SMTP: SMTPConfig{
				Host:     strings.TrimSpace(r.AlertsConfig.SMTPHost),
				Port:     r.AlertsConfig.SMTPPort,
				Username: r.AlertsConfig.SMTPUsername,
				Password: r.AlertsConfig.SMTPPassword,
				From:     strings.TrimSpace(r.AlertsConfig.SMTPFrom),
				TLS:      strings.TrimSpace(r.AlertsConfig.SMTPTLS),
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var problems []string

	for _, section := range []struct {
		name  string
		value interface{}
	}{
		{name: "app", value: c.AppConfig},
		{name: "session", value: c.SessionConfig},
		{name: "db", value: c.DBConfig},
	} {
		if err := validateSection(section.name, section.value); err != nil {
			problems = append(problems, err.Error())
		}
	}

	// OIDC is all-or-nothing: issuer, client_id and client_secret must be set
	// together so SSO is either fully configured or absent.
	oidcSet := 0
	for _, v := range []string{c.OIDCConfig.Issuer, c.OIDCConfig.ClientID, c.OIDCConfig.ClientSecret} {
		if strings.TrimSpace(v) != "" {
			oidcSet++
		}
	}
	if oidcSet != 0 && oidcSet != 3 {
		problems = append(problems, "app.oidc requires issuer, client_id and client_secret to be set together")
	}
	if role := strings.TrimSpace(c.OIDCConfig.DefaultRole); role != "" && !isBuiltinRole(role) {
		problems = append(problems, fmt.Sprintf("app.oidc.default_role must be one of admin|operator|analyst|viewer, got %q", role))
	}

	if c.AppConfig.UseTLS {
		if strings.TrimSpace(c.AppConfig.TLSCertPath) == "" {
			problems = append(problems, "app.http_tls_cert cannot be empty when app.use_tls is true")
		}
		if strings.TrimSpace(c.AppConfig.TLSKeyPath) == "" {
			problems = append(problems, "app.http_tls_key cannot be empty when app.use_tls is true")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(problems, "; "))
	}
	return nil
}

func validateSection(name string, value interface{}) error {
	if err := configValidator.Struct(value); err != nil {
		validationErrors, ok := err.(validator.ValidationErrors)
		if !ok {
			return fmt.Errorf("%s: %w", name, err)
		}

		problems := make([]string, 0, len(validationErrors))
		for _, validationError := range validationErrors {
			problems = append(problems, fmt.Sprintf("%s.%s: %s", name, validationError.Field(), validationError.Tag()))
		}
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

// isBuiltinRole reports whether role is one of the fixed built-in role names.
// Kept local to config to avoid importing core (which imports config).
func isBuiltinRole(role string) bool {
	switch role {
	case "admin", "operator", "analyst", "viewer":
		return true
	default:
		return false
	}
}

func parsePositiveDuration(name, raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: duration must be positive", name)
	}
	return value, nil
}

func loadOrCreateConfigFile(k *koanf.Koanf, configFile string) error {
	if _, err := os.Stat(configFile); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := writeDefaultConfigFile(k, configFile); err != nil {
			return err
		}
		return nil
	}

	if err := k.Load(file.Provider(configFile), toml.Parser()); err != nil {
		return err
	}
	return nil
}

func writeDefaultConfigFile(k *koanf.Koanf, configFile string) error {
	if dir := filepath.Dir(configFile); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
	}

	cfgBytes, err := k.Marshal(toml.Parser())
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}
	if err := os.WriteFile(configFile, cfgBytes, 0o644); err != nil {
		return fmt.Errorf("write default config file: %w", err)
	}
	return nil
}

func loadEnvConfig(k *koanf.Koanf) error {
	return k.Load(env.Provider(envPrefix, ".", func(s string) string {
		key := strings.TrimPrefix(s, envPrefix)
		key = strings.ToLower(key)
		return strings.ReplaceAll(key, "__", ".")
	}), nil)
}

func loadDefaults(k *koanf.Koanf) error {
	key, err := generateSecret()
	if err != nil {
		return fmt.Errorf("generate secure cookie key: %w", err)
	}
	enrollmentKey, err := generateSecret()
	if err != nil {
		return fmt.Errorf("generate enrollment key: %w", err)
	}

	return k.Load(confmap.Provider(map[string]any{
		"app.admin_username":                           "checkpost_admin",
		"app.admin_password":                           "checkpost_password",
		"app.http_tls_cert":                            "server_cert.pem",
		"app.http_tls_key":                             "server_key.pem",
		"app.use_tls":                                  false,
		"app.root_url":                                 "http://localhost:1323",
		"app.secure_cookie_key":                        key,
		"app.enrollment_key":                           enrollmentKey,
		"app.policy_update_interval":                   "1h",
		"app.policy_stale_after":                       "2h",
		"app.oidc.issuer":                              "",
		"app.oidc.client_id":                           "",
		"app.oidc.client_secret":                       "",
		"app.oidc.label":                               "Company SSO",
		"app.oidc.redirect_url":                        "",
		"app.oidc.auth_url":                            "",
		"app.oidc.token_url":                           "",
		"app.oidc.scopes":                              []string{"openid", "profile", "email", "groups"},
		"app.oidc.groups_claim":                        "groups",
		"app.oidc.allowed_domains":                     []string{},
		"app.oidc.auto_create_users":                   true,
		"app.oidc.default_role":                        "",
		"app.session.ttl":                              "8h",
		"db.dbname":                                    "checkpost",
		"db.host":                                      "localhost",
		"db.port":                                      5432,
		"db.password":                                  "checkpost",
		"db.user":                                      "checkpost",
		"results.parquet.enabled":                      true,
		"results.parquet.root":                         "./data/results",
		"results.parquet.duckdb_path":                  "",
		"results.ndjson.enabled":                       false,
		"results.ndjson.path":                          "stdout",
		"results.ndjson.required":                      false,
		"results.clickhouse.enabled":                   false,
		"results.clickhouse.dsn":                       "",
		"results.clickhouse.table":                     "query_results",
		"results.clickhouse.ttl_days":                  0,
		"results.clickhouse.required":                  false,
		"alerts.enabled":                               false,
		"alerts.smtp.host":                             "",
		"alerts.smtp.port":                             587,
		"alerts.smtp.username":                         "",
		"alerts.smtp.password":                         "",
		"alerts.smtp.from":                             "checkpost@example.com",
		"alerts.smtp.tls":                              "starttls",
		"osquery_bootstrap.enabled":                    true,
		"osquery_bootstrap.linux.deb_amd64.url":        "",
		"osquery_bootstrap.linux.deb_amd64.sha256":     "",
		"osquery_bootstrap.linux.deb_arm64.url":        "",
		"osquery_bootstrap.linux.deb_arm64.sha256":     "",
		"osquery_bootstrap.linux.rpm_amd64.url":        "",
		"osquery_bootstrap.linux.rpm_amd64.sha256":     "",
		"osquery_bootstrap.linux.rpm_arm64.url":        "",
		"osquery_bootstrap.linux.rpm_arm64.sha256":     "",
		"osquery_bootstrap.macos.pkg_universal.url":    "",
		"osquery_bootstrap.macos.pkg_universal.sha256": "",
		"osquery_bootstrap.windows.msi_amd64.url":      "",
		"osquery_bootstrap.windows.msi_amd64.sha256":   "",
	}, "."), nil)
}

func packageConfig(url, sha256 string) BootstrapPackage {
	return BootstrapPackage{
		URL:    strings.TrimSpace(url),
		SHA256: strings.TrimSpace(sha256),
	}
}

func generateSecret() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
