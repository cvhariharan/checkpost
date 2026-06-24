package handlers

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/cvhariharan/checkpost/internal/config"
	"github.com/labstack/echo/v4"
)

const (
	linuxFlagfilePath   = "/etc/osquery/osquery.flags"
	linuxSecretPath     = "/etc/osquery/enroll_secret"
	macOSFlagfilePath   = "/private/var/osquery/osquery.flags"
	macOSSecretPath     = "/private/var/osquery/enroll_secret"
	windowsFlagfilePath = `C:\ProgramData\Checkpost\osquery\osquery.flags`
	windowsSecretPath   = `C:\ProgramData\Checkpost\osquery\enroll_secret`
)

type bootstrapTemplateData struct {
	TLSHostname             string
	EnrollmentKey           string
	LinuxTarballAMD64URL    string
	LinuxTarballAMD64SHA256 string
	LinuxTarballARM64URL    string
	LinuxTarballARM64SHA256 string
	MacOSPKGUniversalURL    string
	MacOSPKGUniversalSHA256 string
	WindowsMSIAMD64URL      string
	WindowsMSIAMD64SHA256   string
}

func (h *Handler) HandleOsqueryBootstrap(c echo.Context) error {
	profile, err := h.osqueryBootstrapProfile()
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error rendering osquery bootstrap profile", err, nil)
	}
	noStore(c)
	return c.JSON(http.StatusOK, profile)
}

func (h *Handler) HandleOsqueryBootstrapScript(c echo.Context) error {
	platform := strings.TrimSpace(c.Param("platform"))
	script, contentType, err := h.osqueryBootstrapScript(platform)
	if err != nil {
		return wrapError(http.StatusNotFound, fmt.Sprintf("osquery bootstrap script %s not found", platform), err, nil)
	}
	noStore(c)
	return c.Blob(http.StatusOK, contentType, []byte(script))
}

func (h *Handler) osqueryBootstrapProfile() (OsqueryBootstrapResponse, error) {
	checkpostURL, tlsHostname, warnings := bootstrapURLState(h.cfg.RootURL)
	packages := h.bootstrapPackages()
	warnings = append(warnings, packageWarnings(h.cfg.OsqueryBootstrap, packages)...)

	scripts := make(map[string]string, 3)
	for _, platform := range []string{"linux.sh", "macos.sh", "windows.ps1"} {
		script, _, err := h.osqueryBootstrapScript(platform)
		if err != nil {
			return OsqueryBootstrapResponse{}, err
		}
		scripts[platform] = script
	}

	platforms := []OsqueryBootstrapPlatform{
		{
			Key:               "linux",
			Label:             "Linux",
			Command:           bootstrapCommand(checkpostURL, "linux"),
			ScriptURL:         bootstrapScriptURL(checkpostURL, "linux.sh"),
			VerifyCommand:     "sudo systemctl status osqueryd --no-pager",
			RestartCommand:    "sudo systemctl restart osqueryd && sudo systemctl enable osqueryd",
			Package:           firstPackage(packages["linux"]),
			Packages:          packages["linux"],
			InstallSteps:      []string{"If osquery is not installed, download the generic Linux tarball for the host architecture", "Verify SHA256, then copy osquery into /opt/osquery and link the binaries into /usr/bin", "Install a systemd unit and enable the osqueryd service"},
			FlagfilePath:      linuxFlagfilePath,
			SecretPath:        linuxSecretPath,
			Secret:            h.cfg.EnrollmentKey,
			Flagfile:          osqueryFlagfile(tlsHostname, linuxSecretPath),
			Script:            scripts["linux.sh"],
			ArchitectureNotes: "Uses the generic osquery Linux tarball with amd64 and arm64 entries; works on any distribution running systemd.",
			Caveats:           []string{"Requires systemd and the tar utility on the host."},
		},
		{
			Key:               "macos",
			Label:             "macOS",
			Command:           bootstrapCommand(checkpostURL, "macos"),
			ScriptURL:         bootstrapScriptURL(checkpostURL, "macos.sh"),
			VerifyCommand:     "sudo launchctl print system/io.osquery.agent || sudo launchctl print system/com.facebook.osqueryd",
			RestartCommand:    "sudo osqueryctl restart",
			Package:           firstPackage(packages["macos"]),
			Packages:          packages["macos"],
			InstallSteps:      []string{"If osquery is not installed, download the configured PKG", "Verify SHA256", "Install with the macOS installer command"},
			FlagfilePath:      macOSFlagfilePath,
			SecretPath:        macOSSecretPath,
			Secret:            h.cfg.EnrollmentKey,
			Flagfile:          osqueryFlagfile(tlsHostname, macOSSecretPath),
			Script:            scripts["macos.sh"],
			ArchitectureNotes: "Uses one universal macOS PKG entry.",
			Caveats:           []string{"Service control depends on the LaunchDaemon installed by the osquery package."},
		},
		{
			Key:               "windows",
			Label:             "Windows",
			Command:           bootstrapCommand(checkpostURL, "windows"),
			ScriptURL:         bootstrapScriptURL(checkpostURL, "windows.ps1"),
			VerifyCommand:     "Get-Service osqueryd",
			RestartCommand:    "Restart-Service osqueryd",
			Package:           firstPackage(packages["windows"]),
			Packages:          packages["windows"],
			InstallSteps:      []string{"If osquery is not installed, download the configured MSI", "Verify SHA256", "Install silently with msiexec"},
			FlagfilePath:      windowsFlagfilePath,
			SecretPath:        windowsSecretPath,
			Secret:            h.cfg.EnrollmentKey,
			Flagfile:          osqueryFlagfile(tlsHostname, windowsSecretPath),
			Script:            scripts["windows.ps1"],
			ArchitectureNotes: "Supports Windows amd64 MSI packages in v1.",
			Caveats:           []string{"Run PowerShell as Administrator."},
		},
	}

	return OsqueryBootstrapResponse{
		Ready:        h.cfg.OsqueryBootstrap.Enabled && len(warnings) == 0,
		CheckpostURL: checkpostURL,
		TLSHostname:  tlsHostname,
		Warnings:     warnings,
		Platforms:    platforms,
	}, nil
}

func (h *Handler) osqueryBootstrapScript(platform string) (string, string, error) {
	filename := ""
	contentType := "text/x-shellscript; charset=utf-8"
	switch platform {
	case "linux", "linux.sh":
		filename = "linux.sh.tmpl"
	case "macos", "macos.sh":
		filename = "macos.sh.tmpl"
	case "windows", "windows.ps1":
		filename = "windows.ps1.tmpl"
		contentType = "text/plain; charset=utf-8"
	default:
		return "", "", fmt.Errorf("unknown platform %q", platform)
	}

	raw, err := fs.ReadFile(h.bootstrapTemplates, filename)
	if err != nil {
		return "", "", fmt.Errorf("read bootstrap template: %w", err)
	}

	tmpl, err := template.New(filename).Funcs(template.FuncMap{
		"sh": shellQuote,
		"ps": powershellQuote,
	}).Parse(string(raw))
	if err != nil {
		return "", "", fmt.Errorf("parse bootstrap template: %w", err)
	}

	data := h.bootstrapTemplateData()
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("render bootstrap template: %w", err)
	}
	return buf.String(), contentType, nil
}

func (h *Handler) bootstrapTemplateData() bootstrapTemplateData {
	_, tlsHostname, _ := bootstrapURLState(h.cfg.RootURL)
	cfg := h.cfg.OsqueryBootstrap
	return bootstrapTemplateData{
		TLSHostname:             tlsHostname,
		EnrollmentKey:           h.cfg.EnrollmentKey,
		LinuxTarballAMD64URL:    cfg.Linux.TarballAMD64.URL,
		LinuxTarballAMD64SHA256: cfg.Linux.TarballAMD64.SHA256,
		LinuxTarballARM64URL:    cfg.Linux.TarballARM64.URL,
		LinuxTarballARM64SHA256: cfg.Linux.TarballARM64.SHA256,
		MacOSPKGUniversalURL:    cfg.MacOS.PKGUniversal.URL,
		MacOSPKGUniversalSHA256: cfg.MacOS.PKGUniversal.SHA256,
		WindowsMSIAMD64URL:      cfg.Windows.MSIAMD64.URL,
		WindowsMSIAMD64SHA256:   cfg.Windows.MSIAMD64.SHA256,
	}
}

func bootstrapURLState(raw string) (string, string, []string) {
	var warnings []string
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if value == "" {
		warnings = append(warnings, "app.root_url is empty")
		return "", "", warnings
	}
	if err != nil {
		warnings = append(warnings, "app.root_url cannot be parsed")
		return value, "", warnings
	}
	if parsed.Scheme != "https" {
		warnings = append(warnings, "app.root_url must use https://")
	}
	if parsed.Host == "" {
		warnings = append(warnings, "app.root_url is missing a hostname")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		warnings = append(warnings, "app.root_url must not include a non-root path")
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), parsed.Host, warnings
}

func (h *Handler) bootstrapPackages() map[string][]OsqueryBootstrapPackage {
	cfg := h.cfg.OsqueryBootstrap
	return map[string][]OsqueryBootstrapPackage{
		"linux": {
			packageEntry("linux-tarball-amd64", "Linux tarball amd64", "linux", "tarball", "amd64", "tarball", cfg.Linux.TarballAMD64),
			packageEntry("linux-tarball-arm64", "Linux tarball arm64", "linux", "tarball", "arm64", "tarball", cfg.Linux.TarballARM64),
		},
		"macos": {
			packageEntry("macos-pkg-universal", "macOS PKG universal", "macos", "pkg", "universal", "pkg", cfg.MacOS.PKGUniversal),
		},
		"windows": {
			packageEntry("windows-msi-amd64", "Windows MSI amd64", "windows", "msi", "amd64", "msi", cfg.Windows.MSIAMD64),
		},
	}
}

func packageEntry(key, label, platform, family, arch, format string, pkg config.BootstrapPackage) OsqueryBootstrapPackage {
	return OsqueryBootstrapPackage{
		Key:          key,
		Label:        label,
		Platform:     platform,
		Family:       family,
		Architecture: arch,
		Format:       format,
		URL:          pkg.URL,
		SHA256:       pkg.SHA256,
	}
}

func packageWarnings(cfg config.OsqueryBootstrapConfig, packages map[string][]OsqueryBootstrapPackage) []string {
	var warnings []string
	if !cfg.Enabled {
		warnings = append(warnings, "osquery bootstrap is disabled")
	}
	for _, platform := range []string{"linux", "macos", "windows"} {
		for _, pkg := range packages[platform] {
			if strings.TrimSpace(pkg.URL) == "" {
				warnings = append(warnings, fmt.Sprintf("%s package URL is not configured", pkg.Label))
			}
			if strings.TrimSpace(pkg.SHA256) == "" {
				warnings = append(warnings, fmt.Sprintf("%s package SHA256 is not configured", pkg.Label))
			}
		}
	}
	return warnings
}

func firstPackage(packages []OsqueryBootstrapPackage) OsqueryBootstrapPackage {
	if len(packages) == 0 {
		return OsqueryBootstrapPackage{}
	}
	return packages[0]
}

func bootstrapScriptURL(checkpostURL, script string) string {
	if checkpostURL == "" {
		return "/bootstrap/" + script
	}
	return strings.TrimRight(checkpostURL, "/") + "/bootstrap/" + script
}

func bootstrapCommand(checkpostURL, platform string) string {
	switch platform {
	case "linux":
		return fmt.Sprintf("curl -fsSL %s | sudo bash", bootstrapScriptURL(checkpostURL, "linux.sh"))
	case "macos":
		return fmt.Sprintf("curl -fsSL %s | sudo bash", bootstrapScriptURL(checkpostURL, "macos.sh"))
	case "windows":
		return fmt.Sprintf("powershell -NoProfile -ExecutionPolicy Bypass -Command \"iwr -useb %s | iex\"", bootstrapScriptURL(checkpostURL, "windows.ps1"))
	default:
		return ""
	}
}

func osqueryFlagfile(tlsHostname, secretPath string) string {
	lines := []string{
		"--host_identifier=uuid",
		"--enroll_secret_path=" + secretPath,
		"--tls_hostname=" + tlsHostname,
		"--enroll_tls_endpoint=/api/v1/osquery/enroll",
		"--config_plugin=tls",
		"--config_tls_endpoint=/api/v1/osquery/config",
		"--logger_plugin=tls",
		"--logger_tls_endpoint=/api/v1/osquery/logger",
		"--distributed_plugin=tls",
		"--distributed_tls_read_endpoint=/api/v1/osquery/distributed/read",
		"--distributed_tls_write_endpoint=/api/v1/osquery/distributed/write",
		"--logger_tls_period=10",
		"--distributed_interval=10",
		"--disable_distributed=false",
	}
	return strings.Join(lines, "\n")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func powershellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func noStore(c echo.Context) {
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
}
