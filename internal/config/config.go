package config

type Config struct {
	AppConfig    `koanf:",squash"`
	OIDCConfig   `koanf:",squash"`
	DBConfig     `koanf:",squash"`
	ValkeyConfig `koanf:",squash"`
}

type AppConfig struct {
	AdminUsername   string `koanf:"app.admin_username"`
	AdminPassword   string `koanf:"app.admin_password"`
	TLSCertPath     string `koanf:"app.http_tls_cert"`
	TLSKeyPath      string `koanf:"app.http_tls_key"`
	RootURL         string `koanf:"app.root_url"`
	UseTLS          bool   `koanf:"app.use_tls"`
	SecureCookieKey string `koanf:"app.secure_cookie_key"`
}

type OIDCConfig struct {
	Issuer       string `koanf:"app.oidc.issuer"`
	ClientID     string `koanf:"app.oidc.client_id"`
	ClientSecret string `koanf:"app.oidc.client_secret"`
}

type DBConfig struct {
	DBName   string `koanf:"db.dbname"`
	Host     string `koanf:"db.host"`
	Port     int    `koanf:"db.port"`
	User     string `koanf:"db.user"`
	Password string `koanf:"db.password"`
}

type ValkeyConfig struct {
	Host     string `koanf:"valkey.host"`
	Port     int    `koanf:"valkey.port"`
	Password string `koanf:"valkey.password"`
}
