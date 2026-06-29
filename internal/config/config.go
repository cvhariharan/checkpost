package config

import "time"

type Config struct {
	AppConfig     AppConfig
	OIDCConfig    OIDCConfig
	SessionConfig SessionConfig
	DBConfig      DBConfig
	ResultsConfig ResultsConfig
	AlertsConfig  AlertsConfig
}

type AlertsConfig struct {
	Enabled bool
	SMTP    SMTPConfig
}

type SMTPConfig struct {
	Host     string
	Port     int `validate:"required_with=Host,omitempty,gt=0,lte=65535"`
	Username string
	Password string `validate:"required_with=Username"`
	From     string `validate:"required_with=Host,omitempty,email"`
	TLS      string `validate:"omitempty,oneof=starttls implicit none"` // starttls | implicit | none
}

type AppConfig struct {
	AdminUsername        string `validate:"required"`
	AdminPassword        string `validate:"required"`
	TLSCertPath          string `validate:"required_if=UseTLS true"`
	TLSKeyPath           string `validate:"required_if=UseTLS true"`
	RootURL              string `validate:"required,http_url"`
	UseTLS               bool
	EnrollmentKey        string        `validate:"required"`
	PolicyUpdateInterval time.Duration `validate:"gt=0"`
	PolicyStaleAfter     time.Duration `validate:"gt=0"`
	HeartbeatThreshold   time.Duration
	OsqueryBootstrap     OsqueryBootstrapConfig
}

type OIDCConfig struct {
	Issuer       string `validate:"required_with=ClientID ClientSecret,omitempty,http_url"`
	ClientID     string `validate:"required_with=Issuer ClientSecret"`
	ClientSecret string `validate:"required_with=Issuer ClientID"`

	Label           string
	RedirectURL     string   `validate:"omitempty,http_url"`
	AuthURL         string   `validate:"omitempty,http_url"`
	TokenURL        string   `validate:"omitempty,http_url"`
	Scopes          []string `validate:"omitempty,dive,required"`
	GroupsClaim     string
	AllowedDomains  []string `validate:"omitempty,dive,required"`
	AutoCreateUsers bool
	DefaultRole     string `validate:"omitempty,oneof=admin operator analyst viewer"`
}

// Enabled reports whether SSO is fully configured
func (o OIDCConfig) Enabled() bool {
	return o.Issuer != "" && o.ClientID != "" && o.ClientSecret != ""
}

type SessionConfig struct {
	TTL time.Duration `validate:"gt=0"`
}

type DBConfig struct {
	DBName   string `validate:"required"`
	Host     string `validate:"required"`
	Port     int    `validate:"gt=0,lte=65535"`
	User     string `validate:"required"`
	Password string
}

// ResultsConfig selects which backends receive scheduled-query results.
type ResultsConfig struct {
	// Reader names the backend that powers the in-app results browser. Empty
	// means auto-select (prefer parquet, then clickhouse). The named backend
	// must be enabled; ndjson is write-only and cannot be a reader.
	Reader     string `validate:"omitempty,oneof=parquet clickhouse"`
	Parquet    ParquetConfig
	NDJSON     NDJSONConfig
	ClickHouse ClickHouseConfig
	Adhoc      AdhocConfig
}

type AdhocConfig struct {
	RetentionDays int `validate:"gte=1"`
}

// ParquetConfig is the local Parquet+DuckDB backend that powers the frontend
// results browser. Disable it for external-only logging.
type ParquetConfig struct {
	Enabled    bool
	Root       string `validate:"required_if=Enabled true"`
	DuckDBPath string
}

// NDJSONConfig ships rows as newline-delimited JSON to a file or stdout.
type NDJSONConfig struct {
	Enabled  bool
	Path     string
	Required bool
}

// ClickHouseConfig inserts rows into a ClickHouse table.
type ClickHouseConfig struct {
	Enabled  bool
	DSN      string `validate:"required_if=Enabled true"`
	Table    string
	TTLDays  int `validate:"gte=0"`
	Required bool
}

type OsqueryBootstrapConfig struct {
	Enabled    bool
	Linux      BootstrapPackagesByArch
	MacOS      MacOSBootstrapConfig
	Windows    WindowsBootstrapConfig
	Extensions OsqueryBootstrapExtensionsConfig
}

type BootstrapPackagesByArch struct {
	AMD64 BootstrapPackage
	ARM64 BootstrapPackage
}

type MacOSBootstrapConfig struct {
	PKGUniversal BootstrapPackage
}

type WindowsBootstrapConfig struct {
	MSIAMD64 BootstrapPackage
}

type OsqueryBootstrapExtensionsConfig struct {
	Nftables NftablesExtensionConfig
}

type NftablesExtensionConfig struct {
	Enabled bool
	Linux   BootstrapPackagesByArch
}

type BootstrapPackage struct {
	URL    string `validate:"required_with=SHA256,omitempty,http_url"`
	SHA256 string `validate:"required_with=URL,omitempty,sha256"`
}
