package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/cvhariharan/checkpost/assets"
	"github.com/cvhariharan/checkpost/internal/alerts"
	"github.com/cvhariharan/checkpost/internal/config"
	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/cvhariharan/checkpost/internal/handlers"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/cvhariharan/checkpost/internal/results/clickhouse"
	"github.com/cvhariharan/checkpost/internal/results/ndjson"
	"github.com/cvhariharan/checkpost/internal/results/parquet"
	"github.com/cvhariharan/checkpost/migrations"
	webassets "github.com/cvhariharan/checkpost/web"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

func newServerCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the Checkpost HTTP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(flags)
		},
	}
}

func runServer(flags *rootFlags) error {
	loglevel := slog.LevelError
	if os.Getenv("DEBUG_LOG") == "true" {
		loglevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: loglevel,
	}))
	slog.SetDefault(logger)

	logger.Debug("starting server")

	configFile := flags.config
	if strings.TrimSpace(configFile) == "" {
		configFile = "config.toml"
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.DBConfig.User, cfg.DBConfig.Password, cfg.DBConfig.Host, cfg.DBConfig.Port, cfg.DBConfig.DBName))
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}

	if err := runDBMigration(db); err != nil {
		return fmt.Errorf("could not complete db migration: %w", err)
	}

	store := repo.NewPostgresStore(db)

	resultsSink, resultsReader, err := buildResultsSink(cfg.ResultsConfig, store, logger)
	if err != nil {
		return err
	}
	defer resultsSink.Close()

	rbacModel, err := assets.RBACModel()
	if err != nil {
		return fmt.Errorf("could not load rbac model: %w", err)
	}
	enforcer, err := core.NewEnforcer(db, rbacModel)
	if err != nil {
		return fmt.Errorf("could not initialize casbin enforcer: %w", err)
	}

	c, err := core.NewCore(logger, store, resultsSink, resultsReader, enforcer, cfg.AppConfig)
	if err != nil {
		return fmt.Errorf("could not initialize core: %w", err)
	}

	// Rebuild the shared policy store from the code-defined role matrix +
	// role_bindings, then ensure the config admin exists and holds the admin role.
	if err := c.SyncPolicies(context.Background()); err != nil {
		return fmt.Errorf("could not sync rbac policies: %w", err)
	}
	if err := c.EnsureAdminUser(context.Background(), cfg.AppConfig.AdminUsername, cfg.AppConfig.AdminPassword); err != nil {
		return fmt.Errorf("could not ensure admin user: %w", err)
	}

	tokenSweeper := core.NewTokenSweeper(c, logger)
	tokenSweeper.Start()
	defer tokenSweeper.Close()

	if cfg.AlertsConfig.Enabled {
		core.RegisterAlertSources(store, cfg.AppConfig.PolicyStaleAfter)
		emailTemplates, err := assets.EmailTemplates()
		if err != nil {
			return fmt.Errorf("could not load email templates: %w", err)
		}
		smtpNotifier, err := alerts.NewSMTPNotifier(alerts.SMTPRelay{
			Host:     cfg.AlertsConfig.SMTP.Host,
			Port:     cfg.AlertsConfig.SMTP.Port,
			Username: cfg.AlertsConfig.SMTP.Username,
			Password: cfg.AlertsConfig.SMTP.Password,
			From:     cfg.AlertsConfig.SMTP.From,
			TLS:      cfg.AlertsConfig.SMTP.TLS,
		}, store.ListUserGroupMemberEmails, emailTemplates)
		if err != nil {
			return fmt.Errorf("could not initialize smtp notifier: %w", err)
		}
		alerts.RegisterNotifier(smtpNotifier)
		alerts.RegisterNotifier(alerts.NewWebhookNotifier())

		alertEngine := alerts.NewEngine(store, logger)
		alertEngine.Start()
		defer alertEngine.Close()
	}

	e := echo.New()

	bootstrapTemplates, err := assets.BootstrapTemplates()
	if err != nil {
		return fmt.Errorf("could not load bootstrap templates: %w", err)
	}
	h, err := handlers.NewHandler(logger, db, cfg, c, bootstrapTemplates)
	if err != nil {
		return fmt.Errorf("could not initialize handlers: %w", err)
	}
	e.HTTPErrorHandler = h.ErrorHandler

	// Public auth routes (no session required).
	e.POST("/login", h.HandleLogin)
	e.POST("/logout", h.HandleLogout)
	e.GET("/auth/providers", h.HandleProviders)
	e.GET("/login/oidc", h.HandleOIDCLogin)
	e.GET("/auth/callback", h.HandleAuthCallback)

	// osquery agent endpoints — authenticated by enroll_secret / node_key
	osqueryAPI := e.Group("/api/v1/osquery")
	osqueryAPI.POST("/enroll", h.HandleEnrollment)
	osqueryAPI.POST("/config", h.HandleOSQueryConfig)
	osqueryAPI.POST("/logger", h.HandleLog)
	osqueryAPI.POST("/distributed/read", h.HandleDistributedRead)
	osqueryAPI.POST("/distributed/write", h.HandleDistributedWrite)

	// Everything human goes through Authenticate (+ per-route Authorize).
	api := e.Group("/api/v1", h.Authenticate)

	api.GET("/me", h.HandleMe)

	// Self-service API tokens; minting requires an interactive session.
	api.POST("/auth/tokens", h.HandleIssueToken, h.SessionOnly)
	api.GET("/auth/tokens", h.HandleListTokens)
	api.DELETE("/auth/tokens/:id", h.HandleRevokeToken)

	api.POST("/schedules", h.HandleCreateSchedule, h.Authorize(core.ResourceSchedule, core.ActionCreate))
	api.GET("/schedules", h.HandleSchedulesPagination, h.Authorize(core.ResourceSchedule, core.ActionView))
	api.GET("/schedules/by-name/:name", h.HandleGetScheduleByName, h.Authorize(core.ResourceSchedule, core.ActionView))
	api.GET("/schedules/:id", h.HandleGetSchedule, h.Authorize(core.ResourceSchedule, core.ActionView))
	api.DELETE("/schedules/:id", h.HandleDeleteSchedule, h.Authorize(core.ResourceSchedule, core.ActionDelete))
	api.PUT("/schedules/:id", h.HandleUpdateSchedule, h.Authorize(core.ResourceSchedule, core.ActionUpdate))
	api.GET("/schedules/:id/results", h.HandleScheduleResults, h.Authorize(core.ResourceQueryResult, core.ActionView))

	api.POST("/policies", h.HandleCreatePolicy, h.Authorize(core.ResourcePolicy, core.ActionCreate))
	api.GET("/policies", h.HandlePoliciesPagination, h.Authorize(core.ResourcePolicy, core.ActionView))
	api.GET("/policies/by-name/:name", h.HandleGetPolicyByName, h.Authorize(core.ResourcePolicy, core.ActionView))
	api.GET("/policies/:id", h.HandleGetPolicy, h.Authorize(core.ResourcePolicy, core.ActionView))
	api.DELETE("/policies/:id", h.HandleDeletePolicy, h.Authorize(core.ResourcePolicy, core.ActionDelete))
	api.PUT("/policies/:id", h.HandleUpdatePolicy, h.Authorize(core.ResourcePolicy, core.ActionUpdate))
	api.GET("/policies/:id/machines", h.HandlePolicyMachines, h.Authorize(core.ResourcePolicy, core.ActionView))

	api.POST("/groups", h.HandleCreateGroup, h.Authorize(core.ResourceMachineGroup, core.ActionCreate))
	api.GET("/groups", h.HandleGroupsPagination, h.Authorize(core.ResourceMachineGroup, core.ActionView))
	api.GET("/groups/by-name/:name", h.HandleGetGroupByName, h.Authorize(core.ResourceMachineGroup, core.ActionView))
	api.GET("/groups/:id", h.HandleGetGroup, h.Authorize(core.ResourceMachineGroup, core.ActionView))
	api.DELETE("/groups/:id", h.HandleDeleteGroup, h.Authorize(core.ResourceMachineGroup, core.ActionDelete))
	api.PUT("/groups/:id", h.HandleUpdateGroup, h.Authorize(core.ResourceMachineGroup, core.ActionUpdate))
	api.GET("/groups/:id/machines", h.HandleGroupMachines, h.Authorize(core.ResourceMachineGroup, core.ActionView))
	api.PATCH("/groups/:id/machines", h.HandlePatchGroupMachines, h.Authorize(core.ResourceMachineGroup, core.ActionUpdate))

	api.POST("/owners", h.HandleCreateOwner, h.Authorize(core.ResourceInventory, core.ActionCreate))
	api.GET("/owners", h.HandleOwnersPagination, h.Authorize(core.ResourceInventory, core.ActionView))
	api.GET("/owners/:id", h.HandleGetOwner, h.Authorize(core.ResourceInventory, core.ActionView))
	api.DELETE("/owners/:id", h.HandleDeleteOwner, h.Authorize(core.ResourceInventory, core.ActionDelete))
	api.PUT("/owners/:id", h.HandleUpdateOwner, h.Authorize(core.ResourceInventory, core.ActionUpdate))
	api.GET("/owners/:id/machines", h.HandleOwnerMachines, h.Authorize(core.ResourceInventory, core.ActionView))

	api.GET("/machines", h.HandleMachinesPagination, h.Authorize(core.ResourceMachine, core.ActionView))
	api.GET("/machines/:id/queries", h.HandleMachineQueries, h.Authorize(core.ResourceQueryResult, core.ActionView))
	api.DELETE("/machines/:id/queries/:query_id", h.HandleDeleteMachineQuery, h.Authorize(core.ResourceQueryResult, core.ActionDelete))
	api.GET("/machines/:id/policies", h.HandleMachinePolicies, h.Authorize(core.ResourceMachine, core.ActionView))
	api.GET("/machines/:id/groups", h.HandleMachineGroups, h.Authorize(core.ResourceMachine, core.ActionView))
	api.PUT("/machines/:id/groups", h.HandleReplaceMachineGroups, h.Authorize(core.ResourceMachine, core.ActionUpdate))
	api.GET("/machines/:id/inventory", h.HandleMachineInventory, h.Authorize(core.ResourceInventory, core.ActionView))
	api.PUT("/machines/:id/inventory", h.HandleUpdateMachineInventory, h.Authorize(core.ResourceInventory, core.ActionUpdate))
	api.DELETE("/machines/:id/inventory", h.HandleDeleteMachineInventory, h.Authorize(core.ResourceInventory, core.ActionDelete))
	api.GET("/machines/:id/metrics", h.HandleMachineMetrics, h.Authorize(core.ResourceMachine, core.ActionView))
	api.GET("/metrics/schemas", h.HandleMetricSchemas, h.Authorize(core.ResourceMachine, core.ActionView))
	api.POST("/machines/:id/queries", h.HandleExecuteMachineQuery, h.Authorize(core.ResourceMachine, core.ActionExecute))
	api.GET("/machines/:id", h.HandleGetMachine, h.Authorize(core.ResourceMachine, core.ActionView))

	api.GET("/yara/signature-sources", h.HandleYaraSignatureSources, h.Authorize(core.ResourceYaraSource, core.ActionView))
	api.POST("/yara/signature-sources", h.HandleCreateYaraSignatureSource, h.Authorize(core.ResourceYaraSource, core.ActionCreate))
	api.PUT("/yara/signature-sources/:id", h.HandleUpdateYaraSignatureSource, h.Authorize(core.ResourceYaraSource, core.ActionUpdate))
	api.DELETE("/yara/signature-sources/:id", h.HandleDeleteYaraSignatureSource, h.Authorize(core.ResourceYaraSource, core.ActionDelete))
	api.POST("/yara/scans", h.HandleCreateYaraScan, h.Authorize(core.ResourceYaraScan, core.ActionCreate))
	api.GET("/yara/scans", h.HandleYaraScans, h.Authorize(core.ResourceYaraScan, core.ActionView))
	api.GET("/yara/scans/:id", h.HandleGetYaraScan, h.Authorize(core.ResourceYaraScan, core.ActionView))
	api.GET("/yara/scans/:id/matches", h.HandleYaraScanMatches, h.Authorize(core.ResourceYaraScan, core.ActionView))
	api.GET("/yara/scans/:id/targets", h.HandleYaraScanTargets, h.Authorize(core.ResourceYaraScan, core.ActionView))

	api.POST("/alert-targets", h.HandleCreateAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionCreate))
	api.GET("/alert-targets", h.HandleAlertTargetsPagination, h.Authorize(core.ResourceAlertTarget, core.ActionView))
	api.GET("/alert-targets/by-name/:name", h.HandleGetAlertTargetByName, h.Authorize(core.ResourceAlertTarget, core.ActionView))
	api.GET("/alert-targets/:id", h.HandleGetAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionView))
	api.PUT("/alert-targets/:id", h.HandleUpdateAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionUpdate))
	api.DELETE("/alert-targets/:id", h.HandleDeleteAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionDelete))
	api.POST("/alert-targets/:id/test", h.HandleTestAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionExecute))

	api.POST("/alert-rules", h.HandleCreateAlertRule, h.Authorize(core.ResourceAlertRule, core.ActionCreate))
	api.GET("/alert-rules", h.HandleAlertRulesPagination, h.Authorize(core.ResourceAlertRule, core.ActionView))
	api.GET("/alert-rules/by-name/:name", h.HandleGetAlertRuleByName, h.Authorize(core.ResourceAlertRule, core.ActionView))
	api.GET("/alert-rules/:id", h.HandleGetAlertRule, h.Authorize(core.ResourceAlertRule, core.ActionView))
	api.PUT("/alert-rules/:id", h.HandleUpdateAlertRule, h.Authorize(core.ResourceAlertRule, core.ActionUpdate))
	api.DELETE("/alert-rules/:id", h.HandleDeleteAlertRule, h.Authorize(core.ResourceAlertRule, core.ActionDelete))
	api.GET("/alert-sources", h.HandleAlertSources, h.Authorize(core.ResourceAlertRule, core.ActionView))

	// Admin management routes.
	api.GET("/roles", h.HandleListRoles, h.Authorize(core.ResourceRoleBinding, core.ActionView))

	api.GET("/users", h.HandleListUsers, h.Authorize(core.ResourceUser, core.ActionView))
	api.POST("/users", h.HandleCreateUser, h.Authorize(core.ResourceUser, core.ActionCreate))
	api.PUT("/users/:id", h.HandleUpdateUser, h.Authorize(core.ResourceUser, core.ActionUpdate))
	api.DELETE("/users/:id", h.HandleDeleteUser, h.Authorize(core.ResourceUser, core.ActionDelete))

	api.GET("/user-groups", h.HandleListUserGroups, h.Authorize(core.ResourceUserGroup, core.ActionView))
	api.POST("/user-groups", h.HandleCreateUserGroup, h.Authorize(core.ResourceUserGroup, core.ActionCreate))
	api.GET("/user-groups/:id", h.HandleGetUserGroup, h.Authorize(core.ResourceUserGroup, core.ActionView))
	api.PUT("/user-groups/:id", h.HandleUpdateUserGroup, h.Authorize(core.ResourceUserGroup, core.ActionUpdate))
	api.DELETE("/user-groups/:id", h.HandleDeleteUserGroup, h.Authorize(core.ResourceUserGroup, core.ActionDelete))
	api.GET("/user-groups/:id/members", h.HandleListUserGroupMembers, h.Authorize(core.ResourceUserGroup, core.ActionView))
	api.POST("/user-groups/:id/members", h.HandleAddUserGroupMember, h.Authorize(core.ResourceUserGroup, core.ActionUpdate))
	api.DELETE("/user-groups/:id/members/:user_id", h.HandleRemoveUserGroupMember, h.Authorize(core.ResourceUserGroup, core.ActionUpdate))

	api.GET("/role-bindings", h.HandleListRoleBindings, h.Authorize(core.ResourceRoleBinding, core.ActionView))
	api.POST("/role-bindings", h.HandleCreateRoleBinding, h.Authorize(core.ResourceRoleBinding, core.ActionCreate))
	api.DELETE("/role-bindings/:id", h.HandleDeleteRoleBinding, h.Authorize(core.ResourceRoleBinding, core.ActionDelete))

	e.GET("/bootstrap", h.HandleOsqueryBootstrap, h.Authenticate, h.Authorize(core.ResourceSetting, core.ActionView))
	e.GET("/bootstrap/:platform", h.HandleOsqueryBootstrapScript)

	buildFS, err := fs.Sub(webassets.StaticFiles, "dist")
	if err != nil {
		return fmt.Errorf("could not load embedded frontend: %w", err)
	}
	fileServer := http.FileServer(http.FS(buildFS))
	e.GET("/*", func(c echo.Context) error {
		reqPath := strings.TrimPrefix(c.Request().URL.Path, "/")
		if reqPath != "" {
			if info, err := fs.Stat(buildFS, reqPath); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(c.Response(), c.Request())
				return nil
			}
		}
		indexFile, err := buildFS.Open("index.html")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to open index.html")
		}
		defer indexFile.Close()
		return c.Stream(http.StatusOK, "text/html; charset=utf-8", indexFile)
	})

	if cfg.AppConfig.UseTLS {
		return e.StartTLS(":1323", cfg.AppConfig.TLSCertPath, cfg.AppConfig.TLSKeyPath)
	}
	return e.Start(":1323")
}

// buildResultsSink fans the configured backends into a MultiSink and returns
// the reader that powers the in-app results browser, chosen per rc.Reader
func buildResultsSink(rc config.ResultsConfig, store repo.Store, logger *slog.Logger) (results.Sink, results.Reader, error) {
	sink := results.NewMultiSink(logger)
	var parquetReader, clickhouseReader results.Reader

	if rc.Parquet.Enabled {
		backend, err := parquet.New(rc.Parquet.Root, rc.Parquet.DuckDBPath, store, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("could not initialize parquet backend: %w", err)
		}
		sink.Add(backend, true)
		parquetReader = backend
	}
	if rc.NDJSON.Enabled {
		s, err := ndjson.New(rc.NDJSON.Path, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("could not initialize ndjson backend: %w", err)
		}
		sink.Add(s, rc.NDJSON.Required)
	}
	if rc.ClickHouse.Enabled {
		s, err := clickhouse.New(rc.ClickHouse.DSN, rc.ClickHouse.Table, rc.ClickHouse.TTLDays, store, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("could not initialize clickhouse backend: %w", err)
		}
		sink.Add(s, rc.ClickHouse.Required)
		clickhouseReader = s
	}

	reader, err := selectReader(rc.Reader, parquetReader, clickhouseReader)
	if err != nil {
		return nil, nil, err
	}

	if sink.Len() == 0 {
		logger.Warn("no result backends configured; scheduled-query results will be discarded")
	}
	if reader == nil {
		logger.Warn("no reader-capable result backend enabled; the in-app results browser is unavailable")
	}
	return sink, reader, nil
}

// selectReader resolves the reader choice against the enabled reader-capable
// backends. Empty auto-selects (parquet, then clickhouse)
func selectReader(choice string, parquetReader, clickhouseReader results.Reader) (results.Reader, error) {
	switch choice {
	case "parquet":
		if parquetReader == nil {
			return nil, fmt.Errorf("results.reader is %q but the parquet backend is not enabled", choice)
		}
		return parquetReader, nil
	case "clickhouse":
		if clickhouseReader == nil {
			return nil, fmt.Errorf("results.reader is %q but the clickhouse backend is not enabled", choice)
		}
		return clickhouseReader, nil
	case "":
		if parquetReader != nil {
			return parquetReader, nil
		}
		return clickhouseReader, nil // may be nil when neither is enabled
	default:
		return nil, fmt.Errorf("unknown results.reader %q (want parquet or clickhouse)", choice)
	}
}

func runDBMigration(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver instance: %w", err)
	}

	migrationsFS, err := fs.Sub(migrations.Files, "postgres")
	if err != nil {
		return fmt.Errorf("failed to get migrations sub-filesystem: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create iofs source driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database migration is dirty at version %d", version)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
