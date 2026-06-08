package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cvhariharan/checkpost/internal/config"
	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/zerodha/simplesessions/stores/postgres/v3"
	"github.com/zerodha/simplesessions/v3"
	"golang.org/x/oauth2"
)

// callbackPath is the OIDC redirect path appended to root_url by default.
const callbackPath = "/auth/callback"

// oidcProvider holds the configured OIDC client; nil when SSO is not configured.
type oidcProvider struct {
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	groupsClaim  string
}

type Handler struct {
	cfg      config.AppConfig
	oidcCfg  config.OIDCConfig
	c        *core.Core
	logger   *slog.Logger
	validate *validator.Validate

	sessMgr *simplesessions.Manager
	oidc    *oidcProvider // nil when SSO not configured
}

func getCookie(name string, r interface{}) (*http.Cookie, error) {
	rd := r.(echo.Context)
	return rd.Cookie(name)
}

func setCookie(cookie *http.Cookie, w interface{}) error {
	wr := w.(echo.Context)
	wr.SetCookie(cookie)
	return nil
}

// NewHandler wires the session manager (PostgreSQL store) and OIDC provider and
// returns the HTTP handler. db is required for the session store.
func NewHandler(logger *slog.Logger, db *sql.DB, cfg config.Config, c *core.Core) (*Handler, error) {
	validate := validator.New()

	sessMgr := simplesessions.New(simplesessions.Options{
		EnableAutoCreate: false,
		Cookie: simplesessions.CookieOptions{
			Name:       "checkpost_session",
			IsHTTPOnly: true,
			IsSecure:   cfg.AppConfig.UseTLS,
			SameSite:   http.SameSiteLaxMode,
			MaxAge:     cfg.SessionConfig.TTL,
			Path:       "/",
		},
	})
	sessMgr.SetCookieHooks(getCookie, setCookie)

	sessionStore, err := postgres.New(postgres.Opt{TTL: cfg.SessionConfig.TTL}, db)
	if err != nil {
		return nil, fmt.Errorf("could not initialize postgres session store: %w", err)
	}
	sessMgr.UseStore(sessionStore)

	h := &Handler{
		logger:   logger.WithGroup("handler"),
		cfg:      cfg.AppConfig,
		oidcCfg:  cfg.OIDCConfig,
		c:        c,
		validate: validate,
		sessMgr:  sessMgr,
	}

	if err := h.initOIDC(); err != nil {
		return nil, fmt.Errorf("could not initialize oidc: %w", err)
	}

	return h, nil
}

// initOIDC initializes the OIDC provider. It is a no-op when SSO is unconfigured.
func (h *Handler) initOIDC() error {
	if !h.oidcCfg.Enabled() {
		return nil
	}

	provider, err := oidc.NewProvider(context.Background(), h.oidcCfg.Issuer)
	if err != nil {
		return fmt.Errorf("new oidc provider: %w", err)
	}

	scopes := h.oidcCfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	redirectURL := h.oidcCfg.RedirectURL
	if redirectURL == "" {
		redirectURL, err = url.JoinPath(h.cfg.RootURL, callbackPath)
		if err != nil {
			return fmt.Errorf("build redirect url: %w", err)
		}
	}

	groupsClaim := h.oidcCfg.GroupsClaim
	if groupsClaim == "" {
		groupsClaim = "groups"
	}

	endpoint := provider.Endpoint()
	if h.oidcCfg.AuthURL != "" {
		endpoint.AuthURL = h.oidcCfg.AuthURL
	}
	if h.oidcCfg.TokenURL != "" {
		endpoint.TokenURL = h.oidcCfg.TokenURL
	}

	h.oidc = &oidcProvider{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: h.oidcCfg.ClientID}),
		oauth2Config: &oauth2.Config{
			ClientID:     h.oidcCfg.ClientID,
			ClientSecret: h.oidcCfg.ClientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     endpoint,
			Scopes:       scopes,
		},
		groupsClaim: groupsClaim,
	}
	return nil
}

func (h *Handler) HandlePing(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func showErrorPage(c echo.Context, errCode int, msg string) error {
	return c.Render(errCode, "base.html", IndexPageData{
		Title:        "Machines",
		Active:       "error",
		ErrorCode:    errCode,
		ErrorMessage: msg,
	})
}

func (h *Handler) ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	file := "unknown"
	line := -1
	msg := "error processing the request"
	var customResponse interface{}
	if he, ok := err.(*HTTPError); ok {
		code = he.code
		msg = he.msg
		err = he.err
		file = he.file
		line = he.line
		customResponse = he.customResponse
	}

	h.logger.Error("error processing request",
		"status", code,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"error", err,
		"msg", msg,
		"file", file,
		"line", line,
		"remote_ip", c.RealIP())

	if strings.HasPrefix(c.Request().URL.Path, "/view") {
		if err := showErrorPage(c, code, msg); err != nil {
			h.logger.Error("error showing error page",
				"error", err)
		}
	} else {
		if customResponse != nil {
			c.JSON(code, customResponse)
		} else {
			c.JSON(code, map[string]string{
				"error": msg,
			})
		}
	}
}

func formatValidationErrors(err error) string {
	if err == nil {
		return ""
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return err.Error()
	}

	var errMsgs []string
	for _, e := range validationErrors {
		errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", e.Field(), e.Tag()))
	}

	return strings.Join(errMsgs, "; ")
}
