package handlers

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cvhariharan/checkpost/assets"
	"github.com/cvhariharan/checkpost/internal/config"
	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const testOwnerUUID = "11111111-1111-1111-1111-111111111111"

// fakeEnrollStore satisfies repo.Store, serving the active anonymous and owned
// enrollment secrets the bootstrap handlers read; all other methods are unused.
type fakeEnrollStore struct {
	repo.Store
}

func (fakeEnrollStore) GetActiveAnonymousEnrollmentSecret(context.Context) (repo.EnrollmentSecret, error) {
	return repo.EnrollmentSecret{ID: 1, SecretID: make([]byte, 16), Type: "anonymous"}, nil
}

func (fakeEnrollStore) GetUserByUUID(context.Context, uuid.UUID) (repo.User, error) {
	return repo.User{ID: 1}, nil
}

func (fakeEnrollStore) GetActiveOwnedEnrollmentSecret(context.Context, uuid.NullUUID) (repo.EnrollmentSecret, error) {
	return repo.EnrollmentSecret{ID: 2, SecretID: make([]byte, 16), Type: "owned"}, nil
}

func testBootstrapTemplates(t *testing.T) fs.FS {
	t.Helper()
	fsys, err := assets.BootstrapTemplates()
	if err != nil {
		t.Fatalf("load bootstrap templates: %v", err)
	}
	return fsys
}

// testBootstrapCore builds a minimal Core that can resolve enrollment secrets via
// fakeEnrollStore; sink/reader/enforcer are unused so they stay nil.
func testBootstrapCore(t *testing.T, cfg config.AppConfig) *core.Core {
	t.Helper()
	c, err := core.NewCore(slog.Default(), fakeEnrollStore{}, nil, nil, nil, cfg)
	if err != nil {
		t.Fatalf("new core: %v", err)
	}
	return c
}

func TestOsqueryBootstrapProfileReady(t *testing.T) {
	cfg := testBootstrapConfig("https://checkpost.example.com")
	h := &Handler{cfg: cfg, c: testBootstrapCore(t, cfg), bootstrapTemplates: testBootstrapTemplates(t)}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/bootstrap", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set(contextUserKey, models.SessionUser{UUID: testOwnerUUID, Name: "Test", Email: "test@example.com"})

	if err := h.HandleOsqueryBootstrap(ctx); err != nil {
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
	cfg.OsqueryBootstrap.Linux.AMD64.URL = ""
	h := &Handler{cfg: cfg, c: testBootstrapCore(t, cfg), bootstrapTemplates: testBootstrapTemplates(t)}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/bootstrap", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set(contextUserKey, models.SessionUser{UUID: testOwnerUUID, Name: "Test", Email: "test@example.com"})

	if err := h.HandleOsqueryBootstrap(ctx); err != nil {
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
	if !strings.Contains(joined, "Linux tarball amd64 package URL is not configured") {
		t.Fatalf("warnings = %v, want package URL warning", resp.Warnings)
	}
}

func TestOsqueryBootstrapScript(t *testing.T) {
	cfg := testBootstrapConfig("https://checkpost.example.com")
	h := &Handler{cfg: cfg, c: testBootstrapCore(t, cfg), bootstrapTemplates: testBootstrapTemplates(t)}
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
		"ENROLL_SECRET=${CHECKPOST_ENROLL_SECRET:-'" + core.EnrollmentSecretPrefix,
		"--enroll_tls_endpoint=/api/v1/osquery/enroll",
		"install_osquery_if_missing",
		"TARBALL_AMD64_URL='https://packages.example/osquery'",
		"EXT_ENABLED='true'",
		"EXT_AMD64_URL='https://packages.example/osquery'",
		"rm -f -- \"${EXT_LOAD_PATH}\"",
		"manifest_tmp=\"$(mktemp \"${EXT_LOAD_PATH}.XXXXXX\")\"",
		"mv -f -- \"${manifest_tmp}\" \"${EXT_LOAD_PATH}\"",
		"/etc/systemd/system/osqueryd.service",
		"systemctl enable --now osqueryd",
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
		RootURL:              rootURL,
		EnrollmentSigningKey: "test-signing-key",
		OsqueryBootstrap: config.OsqueryBootstrapConfig{
			Enabled: true,
			Linux: config.BootstrapPackagesByArch{
				AMD64: pkg,
				ARM64: pkg,
			},
			MacOS: config.MacOSBootstrapConfig{
				PKGUniversal: pkg,
			},
			Windows: config.WindowsBootstrapConfig{
				MSIAMD64: pkg,
			},
			Extensions: config.OsqueryBootstrapExtensionsConfig{
				Nftables: config.NftablesExtensionConfig{
					Enabled: true,
					Linux: config.BootstrapPackagesByArch{
						AMD64: pkg,
						ARM64: pkg,
					},
				},
			},
		},
	}
}
