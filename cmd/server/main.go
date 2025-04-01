package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/core"
	"github.com/cvhariharan/watcher/internal/handlers"
	"github.com/cvhariharan/watcher/internal/logqueue"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/goyesql/v2"
	goyesqlx "github.com/knadh/goyesql/v2/sqlx"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
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

	db, err := sqlx.Connect("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", k.String("db.user"), k.String("db.password"), k.String("db.host"), k.Int("db.port"), k.String("db.dbname")))
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	defer db.Close()

	if err := runDBMigration(db); err != nil {
		log.Fatalf("could not complete db migration: %w", err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"localhost:9000"},
		Auth: clickhouse.Auth{
			Database: "watcher",
			Username: "watcher",
			Password: "watcher",
		},
	})
	if err != nil {
		log.Fatalf("could not connect to clickhouse: %w", err)
	}

	if err := runCHMigration(conn); err != nil {
		log.Fatalf("could not complete clickhouse migration: %w", err)
	}

	var pq repo.PreparedQueries
	queries := goyesql.MustParseFile("queries.sql")
	err = goyesqlx.ScanToStruct(&pq, queries, db)
	if err != nil {
		log.Fatalf("could not set up prepared queries: %v", err)
	}

	s := repo.NewPostgresStore(logger, db, pq)

	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		log.Fatal(err)
	}

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{fmt.Sprintf("%s:%d", k.String("valkey.host"), k.Int("valkey.port"))},
		Password: k.String("valkey.password"),
	})
	defer redisClient.Close()

	logqueuer := logqueue.NewStreamLogger(logger, redisClient)

	c := core.NewCore(logger, s, conn, logqueuer)
	e := echo.New()

	h := handlers.NewHandler(logger, cfg.AppConfig, c)
	e.Renderer = handlers.NewTemplateRenderer()
	e.HTTPErrorHandler = h.ErrorHandler

	e.Static("/static", "web/static")

	e.GET("/", func(c echo.Context) error {
		return c.Redirect(301, "/machines")
	})

	e.GET("/machines", h.HandleMachines)
	e.GET("/queries", h.HandleQueries)
	e.GET("/packs", h.HandlePacks)
	e.GET("/schedules", h.HandleSchedules)

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

	osqueryAPI := api.Group("/osquery")
	osqueryAPI.POST("/enroll", h.HandleEnrollment)
	osqueryAPI.POST("/config", h.HandleOSQueryConfig)
	osqueryAPI.POST("/logger", h.HandleLog)

	if cfg.UseTLS {
		e.Logger.Fatal(e.StartTLS(":1323", cfg.AppConfig.TLSCertPath, cfg.AppConfig.TLSKeyPath))
	} else {
		e.Logger.Fatal(e.Start(":1323"))
	}
}

func runDBMigration(db *sqlx.DB) error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver instance: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	// If database is in a dirty state, force the version
	if dirty {
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force migration version: %w", err)
		}
	}

	// Attempt to migrate to the latest version
	if err := m.Up(); err != nil {
		// ErrNoChange means we're at the latest version
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func runCHMigration(conn driver.Conn) error {
	logTable := `
	CREATE TABLE IF NOT EXISTS osquery_results (
    action LowCardinality (String),
    calendar_time String,
    counter UInt64,
    epoch UInt64,
    host_identifier String,
    name String,
    numerics Boolean,
    unix_time DateTime64 (0) DEFAULT 0,
    columns_json String
) ENGINE = MergeTree ()
ORDER BY
    unix_time
PARTITION BY
    toYYYYMM (unix_time);
	`

	if err := conn.Exec(context.Background(), logTable); err != nil {
		return err
	}

	return nil
}

func loadOrCreateConfigFile(configFile string) error {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Println("config file not found, creating default config file")

		// Write the default config to file
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
		"app.admin_username":    "watcher_admin",
		"app.admin_password":    "watcher_password",
		"app.http_tls_cert":     "server_cert.pem",
		"app.http_tls_key":      "server_key.pem",
		"app.use_tls":           false,
		"app.root_url":          "http://localhost:1323",
		"app.secure_cookie_key": hex.EncodeToString(key),
		"app.enrollment_key":    hex.EncodeToString(enrollmentKey),

		"clickhouse.host":     "localhost:9000",
		"clickhouse.db":       "watcher",
		"clickhouse.user":     "watcher",
		"clickhouse.password": "watcher",

		"db.dbname":   "watcher",
		"db.host":     "localhost",
		"db.port":     5432,
		"db.password": "watcher",
		"db.user":     "watcher",

		"valkey.host": "localhost",
		"valkey.port": 6379,
	}, "."), nil)
}
