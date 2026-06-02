package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v4"
)

// Environment variables read by the client subcommands. These are distinct
// from the server's koanf config keys (CHECKPOST_APP__*, CHECKPOST_DB__*, ...): the
// CLI client never reads the server TOML.
const (
	EnvCLIServer = "CHECKPOST_SERVER"
	EnvCLIToken  = "CHECKPOST_TOKEN"
	EnvCLIConfig = "CHECKPOST_CONFIG"
)

// CLIConfig is the client-side configuration for `checkpost apply`: the server to
// talk to, the API token to authenticate with, and client preferences. It is
// authored by hand (the CLI only reads it) and stored as YAML at
// $XDG_CONFIG_HOME/checkpost/config.yaml.
type CLIConfig struct {
	Server   string `yaml:"server"`
	Token    string `yaml:"token"`
	Insecure bool   `yaml:"insecure"`
}

// CLIFlags carries the explicitly-set root flag values for resolution.
type CLIFlags struct {
	Server     string
	Token      string
	ConfigPath string
	Insecure   bool
}

// DefaultCLIConfigPath returns $XDG_CONFIG_HOME/checkpost/config.yaml, falling
// back to ~/.config/checkpost/config.yaml.
func DefaultCLIConfigPath() string {
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Last resort: a relative path. Resolution will simply find no file.
			return filepath.Join(".config", "checkpost", "config.yaml")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "checkpost", "config.yaml")
}

// LoadCLIConfig reads and decodes the CLI config file. A missing file is not an
// error — it returns the zero CLIConfig so env/flags can supply the values.
func LoadCLIConfig(path string) (CLIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CLIConfig{}, nil
		}
		return CLIConfig{}, fmt.Errorf("read cli config %s: %w", path, err)
	}

	var cfg CLIConfig
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return CLIConfig{}, fmt.Errorf("parse cli config %s: %w", path, err)
	}
	return cfg, nil
}

// ResolveCLIConfig produces the effective client configuration from the
// precedence chain: explicit flag → environment variable → config file. A
// warning is written to warn (may be nil) if the config file has loose
// permissions, since it holds a bearer token.
func ResolveCLIConfig(flags CLIFlags, warn io.Writer) (CLIConfig, error) {
	path := firstNonEmpty(flags.ConfigPath, os.Getenv(EnvCLIConfig), DefaultCLIConfigPath())

	if warn != nil {
		if msg := cliConfigPermWarning(path); msg != "" {
			fmt.Fprintln(warn, msg)
		}
	}

	file, err := LoadCLIConfig(path)
	if err != nil {
		return CLIConfig{}, err
	}

	resolved := CLIConfig{
		Server:   firstNonEmpty(flags.Server, os.Getenv(EnvCLIServer), file.Server),
		Token:    firstNonEmpty(flags.Token, os.Getenv(EnvCLIToken), file.Token),
		Insecure: flags.Insecure || file.Insecure,
	}

	if strings.TrimSpace(resolved.Server) == "" {
		return CLIConfig{}, fmt.Errorf("no server configured: pass --server, set %s, or add `server:` to %s", EnvCLIServer, path)
	}
	if strings.TrimSpace(resolved.Token) == "" {
		return CLIConfig{}, fmt.Errorf("no API token configured: pass --token, set %s, or add `token:` to %s (mint one in Settings → API Tokens)", EnvCLIToken, path)
	}

	return resolved, nil
}

// cliConfigPermWarning returns a warning string if the config file exists and
// is readable by group or other (it holds a bearer token; 0600 is expected).
func cliConfigPermWarning(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Sprintf("warning: cli config %s is group/other-readable (%#o); it holds an API token — chmod 0600 it", path, info.Mode().Perm())
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
