package config

import "time"

type Config struct {
	AppConfig  AppConfig
	OIDCConfig OIDCConfig
	DBConfig   DBConfig
	DataConfig DataConfig
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
}

type DBConfig struct {
	DBName   string `validate:"required"`
	Host     string `validate:"required"`
	Port     int    `validate:"gt=0,lte=65535"`
	User     string `validate:"required"`
	Password string
}

type DataConfig struct {
	ParquetRoot string `validate:"required"`
	DuckDBPath  string
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
