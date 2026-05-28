package main

import (
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

	c, err := core.NewCore(logger, store, resultsWriter, resultsReader, cfg.AppConfig)
	if err != nil {
		log.Fatalf("could not initialize core: %v", err)
	}

	e := echo.New()

	h := handlers.NewHandler(logger, cfg.AppConfig, c)
	e.HTTPErrorHandler = h.ErrorHandler

	api := e.Group("/api/v1")

	api.POST("/schedules", h.HandleCreateSchedule)
	api.GET("/schedules", h.HandleSchedulesPagination)
	api.GET("/schedules/:id", h.HandleGetSchedule)
	api.DELETE("/schedules/:id", h.HandleDeleteSchedule)
	api.PUT("/schedules/:id", h.HandleUpdateSchedule)
	api.GET("/schedules/:id/results", h.HandleScheduleResults)

	api.POST("/policies", h.HandleCreatePolicy)
	api.GET("/policies", h.HandlePoliciesPagination)
	api.GET("/policies/:id", h.HandleGetPolicy)
	api.DELETE("/policies/:id", h.HandleDeletePolicy)
	api.PUT("/policies/:id", h.HandleUpdatePolicy)
	api.GET("/policies/:id/machines", h.HandlePolicyMachines)

	api.POST("/groups", h.HandleCreateGroup)
	api.GET("/groups", h.HandleGroupsPagination)
	api.GET("/groups/:id", h.HandleGetGroup)
	api.DELETE("/groups/:id", h.HandleDeleteGroup)
	api.PUT("/groups/:id", h.HandleUpdateGroup)
	api.GET("/groups/:id/machines", h.HandleGroupMachines)
	api.PATCH("/groups/:id/machines", h.HandlePatchGroupMachines)

	api.POST("/owners", h.HandleCreateOwner)
	api.GET("/owners", h.HandleOwnersPagination)
	api.GET("/owners/:id", h.HandleGetOwner)
	api.DELETE("/owners/:id", h.HandleDeleteOwner)
	api.PUT("/owners/:id", h.HandleUpdateOwner)
	api.GET("/owners/:id/machines", h.HandleOwnerMachines)

	api.GET("/machines", h.HandleMachinesPagination)
	api.GET("/machines/:id/queries", h.HandleMachineQueries)
	api.DELETE("/machines/:id/queries/:query_id", h.HandleDeleteMachineQuery)
	api.GET("/machines/:id/policies", h.HandleMachinePolicies)
	api.GET("/machines/:id/groups", h.HandleMachineGroups)
	api.PUT("/machines/:id/groups", h.HandleReplaceMachineGroups)
	api.GET("/machines/:id/inventory", h.HandleMachineInventory)
	api.PUT("/machines/:id/inventory", h.HandleUpdateMachineInventory)
	api.DELETE("/machines/:id/inventory", h.HandleDeleteMachineInventory)
	api.GET("/machines/:id/metrics", h.HandleMachineMetrics)
	api.GET("/metrics/schemas", h.HandleMetricSchemas)
	api.POST("/machines/:id/queries", h.HandleExecuteMachineQuery)
	api.GET("/machines/:id", h.HandleGetMachine)

	osqueryAPI := api.Group("/osquery")
	osqueryAPI.POST("/enroll", h.HandleEnrollment)
	osqueryAPI.POST("/config", h.HandleOSQueryConfig)
	osqueryAPI.POST("/logger", h.HandleLog)
	osqueryAPI.POST("/distributed/read", h.HandleDistributedRead)
	osqueryAPI.POST("/distributed/write", h.HandleDistributedWrite)

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
