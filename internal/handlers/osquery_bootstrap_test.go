package handlers

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cvhariharan/checkpost/assets"
	"github.com/cvhariharan/checkpost/internal/config"
	"github.com/labstack/echo/v4"
)

func testBootstrapTemplates(t *testing.T) fs.FS {
	t.Helper()
	fsys, err := assets.BootstrapTemplates()
	if err != nil {
		t.Fatalf("load bootstrap templates: %v", err)
	}
	return fsys
}

func TestOsqueryBootstrapProfileReady(t *testing.T) {
	h := &Handler{cfg: testBootstrapConfig("https://checkpost.example.com"), bootstrapTemplates: testBootstrapTemplates(t)}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/bootstrap", nil)
	rec := httptest.NewRecorder()

	if err := h.HandleOsqueryBootstrap(e.NewContext(req, rec)); err != nil {
		t.Fatalf("HandleOsqueryBootstrap() error = %v", err)
	}
	if got := rec.Header().Get(echo.HeaderCacheControl); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	var resp OsqueryBootstrapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Ready {
		t.Fatalf("Ready = false, warnings = %v", resp.Warnings)
	}
	if resp.TLSHostname != "checkpost.example.com" {
		t.Fatalf("TLSHostname = %q", resp.TLSHostname)
	}
	if len(resp.Platforms) != 3 {
		t.Fatalf("platform count = %d, want 3", len(resp.Platforms))
	}
	if !strings.Contains(resp.Platforms[0].Command, "/bootstrap/linux.sh") {
		t.Fatalf("linux command = %q, want bootstrap URL", resp.Platforms[0].Command)
	}
}

func TestOsqueryBootstrapProfileWarnings(t *testing.T) {
	cfg := testBootstrapConfig("http://localhost:1323")
	cfg.OsqueryBootstrap.Linux.DEBAMD64.URL = ""
	h := &Handler{cfg: cfg, bootstrapTemplates: testBootstrapTemplates(t)}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/bootstrap", nil)
	rec := httptest.NewRecorder()

	if err := h.HandleOsqueryBootstrap(e.NewContext(req, rec)); err != nil {
		t.Fatalf("HandleOsqueryBootstrap() error = %v", err)
	}

	var resp OsqueryBootstrapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Ready {
		t.Fatal("Ready = true, want false")
	}
	joined := strings.Join(resp.Warnings, "\n")
	if !strings.Contains(joined, "app.root_url must use https://") {
		t.Fatalf("warnings = %v, want root URL warning", resp.Warnings)
	}
	if !strings.Contains(joined, "Linux DEB amd64 package URL is not configured") {
		t.Fatalf("warnings = %v, want package URL warning", resp.Warnings)
	}
}

func TestOsqueryBootstrapScript(t *testing.T) {
	h := &Handler{cfg: testBootstrapConfig("https://checkpost.example.com"), bootstrapTemplates: testBootstrapTemplates(t)}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/bootstrap/linux.sh", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("platform")
	c.SetParamValues("linux.sh")

	if err := h.HandleOsqueryBootstrapScript(c); err != nil {
		t.Fatalf("HandleOsqueryBootstrapScript() error = %v", err)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"TLS_HOSTNAME='checkpost.example.com'",
		"ENROLL_SECRET='enroll-secret'",
		"--enroll_tls_endpoint=/api/v1/osquery/enroll",
		"install_osquery_if_missing",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("script missing %q\n%s", want, body)
		}
	}
	if got := rec.Header().Get(echo.HeaderCacheControl); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func testBootstrapConfig(rootURL string) config.AppConfig {
	pkg := config.BootstrapPackage{URL: "https://packages.example/osquery", SHA256: strings.Repeat("a", 64)}
	return config.AppConfig{
		RootURL:       rootURL,
		EnrollmentKey: "enroll-secret",
		OsqueryBootstrap: config.OsqueryBootstrapConfig{
			Enabled: true,
			Linux: config.LinuxBootstrapConfig{
				DEBAMD64: pkg,
				DEBARM64: pkg,
				RPMAMD64: pkg,
				RPMARM64: pkg,
			},
			MacOS: config.MacOSBootstrapConfig{
				PKGUniversal: pkg,
			},
			Windows: config.WindowsBootstrapConfig{
				MSIAMD64: pkg,
			},
		},
	}
}
