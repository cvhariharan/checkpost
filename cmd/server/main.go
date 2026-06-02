package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/cvhariharan/watcher/internal/alerts"
	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/core"
	"github.com/cvhariharan/watcher/internal/handlers"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/cvhariharan/watcher/internal/results"
	"github.com/cvhariharan/watcher/migrations"
	webassets "github.com/cvhariharan/watcher/web"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

func main() {
	loglevel := slog.LevelError
	if os.Getenv("DEBUG_LOG") == "true" {
		loglevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: loglevel,
	}))
	slog.SetDefault(logger)

	logger.Debug("starting server")

	var configFile string
	flag.StringVar(&configFile, "config", "config.toml", "Path to the config file. If the file doesn't exist, a default file will be generated")
	flag.Parse()

	cfg, err := config.Load(configFile)
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.DBConfig.User, cfg.DBConfig.Password, cfg.DBConfig.Host, cfg.DBConfig.Port, cfg.DBConfig.DBName))
	if err != nil {
		log.Fatalf("could not open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}

	if err := runDBMigration(db); err != nil {
		log.Fatalf("could not complete db migration: %v", err)
	}

	store := repo.NewPostgresStore(db)

	resultsWriter, err := results.NewWriter(cfg.DataConfig.ParquetRoot, results.NewQueryStore(store), logger)
	if err != nil {
		log.Fatalf("could not initialize results writer: %v", err)
	}
	defer resultsWriter.Close()

	resultsReader, err := results.NewReader(cfg.DataConfig.ParquetRoot, cfg.DataConfig.DuckDBPath)
	if err != nil {
		log.Fatalf("could not initialize results reader: %v", err)
	}
	defer resultsReader.Close()

	resultsWorkers := results.NewWorkers(cfg.DataConfig.ParquetRoot, store, resultsReader, logger)
	resultsWorkers.Start()
	defer resultsWorkers.Close()

	enforcer, err := core.NewEnforcer(db)
	if err != nil {
		log.Fatalf("could not initialize casbin enforcer: %v", err)
	}

	c, err := core.NewCore(logger, store, resultsWriter, resultsReader, enforcer, cfg.AppConfig)
	if err != nil {
		log.Fatalf("could not initialize core: %v", err)
	}

	// Rebuild the shared policy store from the code-defined role matrix +
	// role_bindings, then ensure the config admin exists and holds the admin role.
	if err := c.SyncPolicies(context.Background()); err != nil {
		log.Fatalf("could not sync rbac policies: %v", err)
	}
	if err := c.EnsureAdminUser(context.Background(), cfg.AppConfig.AdminUsername, cfg.AppConfig.AdminPassword); err != nil {
		log.Fatalf("could not ensure admin user: %v", err)
	}

	tokenSweeper := core.NewTokenSweeper(c, logger)
	tokenSweeper.Start()
	defer tokenSweeper.Close()

	if cfg.AlertsConfig.Enabled {
		core.RegisterAlertSources(store, cfg.AppConfig.PolicyStaleAfter)
		smtpNotifier, err := alerts.NewSMTPNotifier(alerts.SMTPRelay{
			Host:     cfg.AlertsConfig.SMTP.Host,
			Port:     cfg.AlertsConfig.SMTP.Port,
			Username: cfg.AlertsConfig.SMTP.Username,
			Password: cfg.AlertsConfig.SMTP.Password,
			From:     cfg.AlertsConfig.SMTP.From,
			TLS:      cfg.AlertsConfig.SMTP.TLS,
		}, store.ListUserGroupMemberEmails)
		if err != nil {
			log.Fatalf("could not initialize smtp notifier: %v", err)
		}
		alerts.RegisterNotifier(smtpNotifier)
		alerts.RegisterNotifier(alerts.NewWebhookNotifier())

		alertEngine := alerts.NewEngine(store, logger)
		alertEngine.Start()
		defer alertEngine.Close()
	}

	e := echo.New()

	h, err := handlers.NewHandler(logger, db, cfg, c)
	if err != nil {
		log.Fatalf("could not initialize handlers: %v", err)
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
	api.GET("/schedules/:id", h.HandleGetSchedule, h.Authorize(core.ResourceSchedule, core.ActionView))
	api.DELETE("/schedules/:id", h.HandleDeleteSchedule, h.Authorize(core.ResourceSchedule, core.ActionDelete))
	api.PUT("/schedules/:id", h.HandleUpdateSchedule, h.Authorize(core.ResourceSchedule, core.ActionUpdate))
	api.GET("/schedules/:id/results", h.HandleScheduleResults, h.Authorize(core.ResourceQueryResult, core.ActionView))

	api.POST("/policies", h.HandleCreatePolicy, h.Authorize(core.ResourcePolicy, core.ActionCreate))
	api.GET("/policies", h.HandlePoliciesPagination, h.Authorize(core.ResourcePolicy, core.ActionView))
	api.GET("/policies/:id", h.HandleGetPolicy, h.Authorize(core.ResourcePolicy, core.ActionView))
	api.DELETE("/policies/:id", h.HandleDeletePolicy, h.Authorize(core.ResourcePolicy, core.ActionDelete))
	api.PUT("/policies/:id", h.HandleUpdatePolicy, h.Authorize(core.ResourcePolicy, core.ActionUpdate))
	api.GET("/policies/:id/machines", h.HandlePolicyMachines, h.Authorize(core.ResourcePolicy, core.ActionView))

	api.POST("/groups", h.HandleCreateGroup, h.Authorize(core.ResourceMachineGroup, core.ActionCreate))
	api.GET("/groups", h.HandleGroupsPagination, h.Authorize(core.ResourceMachineGroup, core.ActionView))
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
	api.GET("/alert-targets/:id", h.HandleGetAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionView))
	api.PUT("/alert-targets/:id", h.HandleUpdateAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionUpdate))
	api.DELETE("/alert-targets/:id", h.HandleDeleteAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionDelete))
	api.POST("/alert-targets/:id/test", h.HandleTestAlertTarget, h.Authorize(core.ResourceAlertTarget, core.ActionExecute))

	api.POST("/alert-rules", h.HandleCreateAlertRule, h.Authorize(core.ResourceAlertRule, core.ActionCreate))
	api.GET("/alert-rules", h.HandleAlertRulesPagination, h.Authorize(core.ResourceAlertRule, core.ActionView))
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
		log.Fatalf("could not load embedded frontend: %v", err)
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
		e.Logger.Fatal(e.StartTLS(":1323", cfg.AppConfig.TLSCertPath, cfg.AppConfig.TLSKeyPath))
	} else {
		e.Logger.Fatal(e.Start(":1323"))
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
