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

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/core"
	"github.com/cvhariharan/watcher/internal/handlers"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/cvhariharan/watcher/internal/results"
	webassets "github.com/cvhariharan/watcher/web"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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

	e := echo.New()

	h, err := handlers.NewHandler(logger, db, cfg, c)
	if err != nil {
		log.Fatalf("could not initialize handlers: %v", err)
	}
	e.HTTPErrorHandler = h.ErrorHandler

	// Resource/action constants for per-route authorization.
	const (
		rMachine      = "machine"
		rMachineGroup = "machine_group"
		rPolicy       = "policy"
		rSchedule     = "schedule"
		rQueryResult  = "query_result"
		rYaraSource   = "yara_source"
		rYaraScan     = "yara_scan"
		rInventory    = "inventory"
		rUser         = "user"
		rUserGroup    = "user_group"
		rRoleBinding  = "role_binding"
		rSetting      = "setting"

		aView    = "view"
		aCreate  = "create"
		aUpdate  = "update"
		aDelete  = "delete"
		aExecute = "execute"
	)

	// Public auth routes (no session required).
	e.POST("/login", h.HandleLogin)
	e.POST("/logout", h.HandleLogout)
	e.GET("/auth/providers", h.HandleProviders)
	e.GET("/login/oidc", h.HandleOIDCLogin)
	e.GET("/auth/callback", h.HandleAuthCallback)

	// osquery agent endpoints — authenticated by enroll_secret / node_key, NOT
	// the human session/RBAC layer.
	osqueryAPI := e.Group("/api/v1/osquery")
	osqueryAPI.POST("/enroll", h.HandleEnrollment)
	osqueryAPI.POST("/config", h.HandleOSQueryConfig)
	osqueryAPI.POST("/logger", h.HandleLog)
	osqueryAPI.POST("/distributed/read", h.HandleDistributedRead)
	osqueryAPI.POST("/distributed/write", h.HandleDistributedWrite)

	// Everything human goes through Authenticate (+ per-route Authorize).
	api := e.Group("/api/v1", h.Authenticate)

	api.GET("/me", h.HandleMe)

	api.POST("/schedules", h.HandleCreateSchedule, h.Authorize(rSchedule, aCreate))
	api.GET("/schedules", h.HandleSchedulesPagination, h.Authorize(rSchedule, aView))
	api.GET("/schedules/:id", h.HandleGetSchedule, h.Authorize(rSchedule, aView))
	api.DELETE("/schedules/:id", h.HandleDeleteSchedule, h.Authorize(rSchedule, aDelete))
	api.PUT("/schedules/:id", h.HandleUpdateSchedule, h.Authorize(rSchedule, aUpdate))
	api.GET("/schedules/:id/results", h.HandleScheduleResults, h.Authorize(rQueryResult, aView))

	api.POST("/policies", h.HandleCreatePolicy, h.Authorize(rPolicy, aCreate))
	api.GET("/policies", h.HandlePoliciesPagination, h.Authorize(rPolicy, aView))
	api.GET("/policies/:id", h.HandleGetPolicy, h.Authorize(rPolicy, aView))
	api.DELETE("/policies/:id", h.HandleDeletePolicy, h.Authorize(rPolicy, aDelete))
	api.PUT("/policies/:id", h.HandleUpdatePolicy, h.Authorize(rPolicy, aUpdate))
	api.GET("/policies/:id/machines", h.HandlePolicyMachines, h.Authorize(rPolicy, aView))

	api.POST("/groups", h.HandleCreateGroup, h.Authorize(rMachineGroup, aCreate))
	api.GET("/groups", h.HandleGroupsPagination, h.Authorize(rMachineGroup, aView))
	api.GET("/groups/:id", h.HandleGetGroup, h.Authorize(rMachineGroup, aView))
	api.DELETE("/groups/:id", h.HandleDeleteGroup, h.Authorize(rMachineGroup, aDelete))
	api.PUT("/groups/:id", h.HandleUpdateGroup, h.Authorize(rMachineGroup, aUpdate))
	api.GET("/groups/:id/machines", h.HandleGroupMachines, h.Authorize(rMachineGroup, aView))
	api.PATCH("/groups/:id/machines", h.HandlePatchGroupMachines, h.Authorize(rMachineGroup, aUpdate))

	api.POST("/owners", h.HandleCreateOwner, h.Authorize(rInventory, aCreate))
	api.GET("/owners", h.HandleOwnersPagination, h.Authorize(rInventory, aView))
	api.GET("/owners/:id", h.HandleGetOwner, h.Authorize(rInventory, aView))
	api.DELETE("/owners/:id", h.HandleDeleteOwner, h.Authorize(rInventory, aDelete))
	api.PUT("/owners/:id", h.HandleUpdateOwner, h.Authorize(rInventory, aUpdate))
	api.GET("/owners/:id/machines", h.HandleOwnerMachines, h.Authorize(rInventory, aView))

	api.GET("/machines", h.HandleMachinesPagination, h.Authorize(rMachine, aView))
	api.GET("/machines/:id/queries", h.HandleMachineQueries, h.Authorize(rQueryResult, aView))
	api.DELETE("/machines/:id/queries/:query_id", h.HandleDeleteMachineQuery, h.Authorize(rQueryResult, aDelete))
	api.GET("/machines/:id/policies", h.HandleMachinePolicies, h.Authorize(rMachine, aView))
	api.GET("/machines/:id/groups", h.HandleMachineGroups, h.Authorize(rMachine, aView))
	api.PUT("/machines/:id/groups", h.HandleReplaceMachineGroups, h.Authorize(rMachine, aUpdate))
	api.GET("/machines/:id/inventory", h.HandleMachineInventory, h.Authorize(rInventory, aView))
	api.PUT("/machines/:id/inventory", h.HandleUpdateMachineInventory, h.Authorize(rInventory, aUpdate))
	api.DELETE("/machines/:id/inventory", h.HandleDeleteMachineInventory, h.Authorize(rInventory, aDelete))
	api.GET("/machines/:id/metrics", h.HandleMachineMetrics, h.Authorize(rMachine, aView))
	api.GET("/metrics/schemas", h.HandleMetricSchemas, h.Authorize(rMachine, aView))
	api.POST("/machines/:id/queries", h.HandleExecuteMachineQuery, h.Authorize(rMachine, aExecute))
	api.GET("/machines/:id", h.HandleGetMachine, h.Authorize(rMachine, aView))

	api.GET("/yara/signature-sources", h.HandleYaraSignatureSources, h.Authorize(rYaraSource, aView))
	api.POST("/yara/signature-sources", h.HandleCreateYaraSignatureSource, h.Authorize(rYaraSource, aCreate))
	api.PUT("/yara/signature-sources/:id", h.HandleUpdateYaraSignatureSource, h.Authorize(rYaraSource, aUpdate))
	api.DELETE("/yara/signature-sources/:id", h.HandleDeleteYaraSignatureSource, h.Authorize(rYaraSource, aDelete))
	api.POST("/yara/scans", h.HandleCreateYaraScan, h.Authorize(rYaraScan, aCreate))
	api.GET("/yara/scans", h.HandleYaraScans, h.Authorize(rYaraScan, aView))
	api.GET("/yara/scans/:id", h.HandleGetYaraScan, h.Authorize(rYaraScan, aView))
	api.GET("/yara/scans/:id/matches", h.HandleYaraScanMatches, h.Authorize(rYaraScan, aView))
	api.GET("/yara/scans/:id/targets", h.HandleYaraScanTargets, h.Authorize(rYaraScan, aView))

	// Admin management routes.
	api.GET("/roles", h.HandleListRoles, h.Authorize(rRoleBinding, aView))

	api.GET("/users", h.HandleListUsers, h.Authorize(rUser, aView))
	api.POST("/users", h.HandleCreateUser, h.Authorize(rUser, aCreate))
	api.PUT("/users/:id", h.HandleUpdateUser, h.Authorize(rUser, aUpdate))
	api.DELETE("/users/:id", h.HandleDeleteUser, h.Authorize(rUser, aDelete))

	api.GET("/user-groups", h.HandleListUserGroups, h.Authorize(rUserGroup, aView))
	api.POST("/user-groups", h.HandleCreateUserGroup, h.Authorize(rUserGroup, aCreate))
	api.GET("/user-groups/:id", h.HandleGetUserGroup, h.Authorize(rUserGroup, aView))
	api.PUT("/user-groups/:id", h.HandleUpdateUserGroup, h.Authorize(rUserGroup, aUpdate))
	api.DELETE("/user-groups/:id", h.HandleDeleteUserGroup, h.Authorize(rUserGroup, aDelete))
	api.GET("/user-groups/:id/members", h.HandleListUserGroupMembers, h.Authorize(rUserGroup, aView))
	api.POST("/user-groups/:id/members", h.HandleAddUserGroupMember, h.Authorize(rUserGroup, aUpdate))
	api.DELETE("/user-groups/:id/members/:user_id", h.HandleRemoveUserGroupMember, h.Authorize(rUserGroup, aUpdate))

	api.GET("/role-bindings", h.HandleListRoleBindings, h.Authorize(rRoleBinding, aView))
	api.POST("/role-bindings", h.HandleCreateRoleBinding, h.Authorize(rRoleBinding, aCreate))
	api.DELETE("/role-bindings/:id", h.HandleDeleteRoleBinding, h.Authorize(rRoleBinding, aDelete))

	// The JSON bootstrap profile contains the enroll secret → behind setting:view.
	// The raw install script (curl | bash) cannot carry a session cookie → public
	// (network-restrict in production; the enroll secret is already shared).
	e.GET("/bootstrap", h.HandleOsqueryBootstrap, h.Authenticate, h.Authorize(rSetting, aView))
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

	m, err := migrate.NewWithDatabaseInstance("file://migrations/postgres", "postgres", driver)
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
