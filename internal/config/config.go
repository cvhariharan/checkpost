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
	Port     int
	Username string
	Password string
	From     string
	TLS      string // starttls | implicit | none
}

type AppConfig struct {
	AdminUsername        string `validate:"required"`
	AdminPassword        string `validate:"required"`
	TLSCertPath          string
	TLSKeyPath           string
	RootURL              string `validate:"required,url"`
	UseTLS               bool
	SecureCookieKey      string        `validate:"required"`
	EnrollmentKey        string        `validate:"required"`
	PolicyUpdateInterval time.Duration `validate:"required"`
	PolicyStaleAfter     time.Duration `validate:"required"`
	OsqueryBootstrap     OsqueryBootstrapConfig
}

type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string

	Label           string
	RedirectURL     string
	AuthURL         string
	TokenURL        string
	Scopes          []string
	GroupsClaim     string
	AllowedDomains  []string
	AutoCreateUsers bool
	DefaultRole     string
}

// Enabled reports whether SSO is fully configured (all-or-nothing).
func (o OIDCConfig) Enabled() bool {
	return o.Issuer != "" && o.ClientID != "" && o.ClientSecret != ""
}

type SessionConfig struct {
	TTL time.Duration `validate:"required"`
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
	Parquet    ParquetConfig
	NDJSON     NDJSONConfig
	ClickHouse ClickHouseConfig
}

// ParquetConfig is the local Parquet+DuckDB backend that powers the frontend
// results browser. Disable it for external-only logging.
type ParquetConfig struct {
	Enabled    bool
	Root       string
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
	DSN      string
	Table    string
	TTLDays  int
	Required bool
}

type OsqueryBootstrapConfig struct {
	Enabled bool
	Linux   LinuxBootstrapConfig
	MacOS   MacOSBootstrapConfig
	Windows WindowsBootstrapConfig
}

type LinuxBootstrapConfig struct {
	DEBAMD64 BootstrapPackage
	DEBARM64 BootstrapPackage
	RPMAMD64 BootstrapPackage
	RPMARM64 BootstrapPackage
}

type MacOSBootstrapConfig struct {
	PKGUniversal BootstrapPackage
}

type WindowsBootstrapConfig struct {
	MSIAMD64 BootstrapPackage
}

type BootstrapPackage struct {
	URL    string
	SHA256 string
}
