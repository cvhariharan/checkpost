package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

var k = koanf.New(".")

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

	if err := setDefaultConfig(); err != nil {
		log.Fatalf("error creating default config: %v", err)
	}

	if err := loadOrCreateConfigFile(configFile); err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	if err := loadEnvConfig(); err != nil {
		log.Fatalf("could not load env config: %v", err)
	}

	db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", k.String("db.user"), k.String("db.password"), k.String("db.host"), k.Int("db.port"), k.String("db.dbname")))
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

	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		log.Fatal(err)
	}

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

	api.POST("/query", h.HandleCreateQuery)
	api.GET("/query/:id", h.HandleGetQuery)
	api.DELETE("/query/:id", h.HandleDeleteQuery)
	api.PUT("/query/:id", h.HandleUpdateQuery)
	api.GET("/queries", h.HandleQueriesPagination)

	api.POST("/schedule", h.HandleCreateSchedule)
	api.GET("/schedules", h.HandleSchedulesPagination)
	api.GET("/schedule/:id", h.HandleGetSchedule)
	api.DELETE("/schedule/:id", h.HandleDeleteSchedule)
	api.PUT("/schedule/:id", h.HandleUpdateSchedule)
	api.GET("/schedule/:id/results", h.HandleScheduleResults)

	api.POST("/policy", h.HandleCreatePolicy)
	api.GET("/policies", h.HandlePoliciesPagination)
	api.GET("/policy/:id", h.HandleGetPolicy)
	api.DELETE("/policy/:id", h.HandleDeletePolicy)
	api.PUT("/policy/:id", h.HandleUpdatePolicy)
	api.GET("/policy/:id/machines", h.HandlePolicyMachines)

	api.POST("/group", h.HandleCreateGroup)
	api.GET("/groups", h.HandleGroupsPagination)
	api.GET("/group/:id", h.HandleGetGroup)
	api.DELETE("/group/:id", h.HandleDeleteGroup)
	api.PUT("/group/:id", h.HandleUpdateGroup)
	api.GET("/group/:id/machines", h.HandleGroupMachines)

	api.GET("/machines", h.HandleMachinesPagination)
	api.GET("/machines/:id/queries", h.HandleMachineQueries)
	api.DELETE("/machines/:id/queries/:query_id", h.HandleDeleteMachineQuery)
	api.GET("/machines/:id/policies", h.HandleMachinePolicies)
	api.GET("/machines/:id/groups", h.HandleMachineGroups)
	api.PUT("/machines/:id/groups", h.HandleReplaceMachineGroups)
	api.POST("/machines/:id/query", h.HandleExecuteMachineQuery)
	api.GET("/machines/:id", h.HandleGetMachine)

	api.GET("/packs", h.HandlePacksList)

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

	if cfg.UseTLS {
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

func loadOrCreateConfigFile(configFile string) error {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Println("config file not found, creating default config file")

		f, err := os.Create(configFile)
		if err != nil {
			return fmt.Errorf("error creating config file: %v", err)
		}
		defer f.Close()

		cfgBytes, err := k.Marshal(toml.Parser())
		if err != nil {
			return fmt.Errorf("could not marshal default config: %v", err)
		}

		if _, err := f.Write(cfgBytes); err != nil {
			return fmt.Errorf("could not write default config file: %v", err)
		}

		return nil
	}

	if err := k.Load(file.Provider(configFile), toml.Parser()); err != nil {
		log.Fatalf("error loading config: %v", err)
	}
	return nil
}

func loadEnvConfig() error {
	const prefix = "WATCHER_"

	return k.Load(env.Provider(prefix, ".", func(s string) string {
		key := strings.TrimPrefix(s, prefix)
		key = strings.ToLower(key)
		return strings.ReplaceAll(key, "__", ".")
	}), nil)
}

func setDefaultConfig() error {
	key := make([]byte, 16)
	enrollmentKey := make([]byte, 16)

	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("could not generate random key for securecookie encryption: %w", err)
	}

	if _, err := rand.Read(enrollmentKey); err != nil {
		return fmt.Errorf("could not generate random enrollment key for osquery: %w", err)
	}

	return k.Load(confmap.Provider(map[string]any{
		"app.admin_username":         "watcher_admin",
		"app.admin_password":         "watcher_password",
		"app.http_tls_cert":          "server_cert.pem",
		"app.http_tls_key":           "server_key.pem",
		"app.use_tls":                false,
		"app.root_url":               "http://localhost:1323",
		"app.secure_cookie_key":      hex.EncodeToString(key),
		"app.enrollment_key":         hex.EncodeToString(enrollmentKey),
		"app.policy_update_interval": "1h",
		"app.policy_stale_after":     "2h",

		"db.dbname":   "watcher",
		"db.host":     "localhost",
		"db.port":     5432,
		"db.password": "watcher",
		"db.user":     "watcher",

		"data.parquet_root": "./data/results",
		"data.duckdb_path":  "",
	}, "."), nil)
}
