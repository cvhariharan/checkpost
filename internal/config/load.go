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

const envPrefix = "WATCHER_"

var (
	defaultPolicyUpdateInterval = time.Hour
	defaultPolicyStaleAfter     = 2 * time.Hour
	configValidator             = validator.New()
)

type rawConfig struct {
	AppConfig  rawAppConfig  `koanf:",squash"`
	OIDCConfig rawOIDCConfig `koanf:",squash"`
	DBConfig   rawDBConfig   `koanf:",squash"`
	DataConfig rawDataConfig `koanf:",squash"`
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
	Issuer       string `koanf:"app.oidc.issuer"`
	ClientID     string `koanf:"app.oidc.client_id"`
	ClientSecret string `koanf:"app.oidc.client_secret"`
}

type rawDBConfig struct {
	DBName   string `koanf:"db.dbname"`
	Host     string `koanf:"db.host"`
	Port     int    `koanf:"db.port"`
	User     string `koanf:"db.user"`
	Password string `koanf:"db.password"`
}

type rawDataConfig struct {
	ParquetRoot string `koanf:"data.parquet_root"`
	DuckDBPath  string `koanf:"data.duckdb_path"`
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
		},
		OIDCConfig: OIDCConfig{
			Issuer:       r.OIDCConfig.Issuer,
			ClientID:     r.OIDCConfig.ClientID,
			ClientSecret: r.OIDCConfig.ClientSecret,
		},
		DBConfig: DBConfig{
			DBName:   r.DBConfig.DBName,
			Host:     r.DBConfig.Host,
			Port:     r.DBConfig.Port,
			User:     r.DBConfig.User,
			Password: r.DBConfig.Password,
		},
		DataConfig: DataConfig{
			ParquetRoot: r.DataConfig.ParquetRoot,
			DuckDBPath:  r.DataConfig.DuckDBPath,
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
		{name: "db", value: c.DBConfig},
		{name: "data", value: c.DataConfig},
	} {
		if err := validateSection(section.name, section.value); err != nil {
			problems = append(problems, err.Error())
		}
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
		"app.admin_username":         "watcher_admin",
		"app.admin_password":         "watcher_password",
		"app.http_tls_cert":          "server_cert.pem",
		"app.http_tls_key":           "server_key.pem",
		"app.use_tls":                false,
		"app.root_url":               "http://localhost:1323",
		"app.secure_cookie_key":      key,
		"app.enrollment_key":         enrollmentKey,
		"app.policy_update_interval": "1h",
		"app.policy_stale_after":     "2h",
		"db.dbname":                  "watcher",
		"db.host":                    "localhost",
		"db.port":                    5432,
		"db.password":                "watcher",
		"db.user":                    "watcher",
		"data.parquet_root":          "./data/results",
		"data.duckdb_path":           "",
	}, "."), nil)
}

func generateSecret() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
