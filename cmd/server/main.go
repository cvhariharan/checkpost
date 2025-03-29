package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/core"
	"github.com/cvhariharan/watcher/internal/handlers"
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
)

var k = koanf.New(".")

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.SetLogLoggerLevel(slog.LevelError)
	if os.Getenv("DEBUG_LOG") == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	var configFile string
	flag.StringVar(&configFile, "config", "config.toml", "Path to the config file. If the file doesn't exist, a default file will be generated")
	flag.Parse()

	if err := setDefaultConfig(); err != nil {
		log.Fatalf("error creating default config: %v", err)
	}

	if err := loadOrCreateConfigFile(configFile); err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	if err := initDB(); err != nil {
		log.Fatalf("could not initialize db: %v", err)
	}

	db, err := sqlx.Connect("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", k.String("db.user"), k.String("db.password"), k.String("db.host"), k.Int("db.port"), k.String("db.dbname")))
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	defer db.Close()

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

	c := core.NewCore(logger, s)
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

	osqueryAPI := api.Group("/osquery")
	osqueryAPI.POST("/enroll", h.HandleEnrollment)

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

func initDB() error {
	db, err := sqlx.Connect("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", k.String("db.user"), k.String("db.password"), k.String("db.host"), k.Int("db.port"), k.String("db.dbname")))
	if err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}
	defer db.Close()

	if err := runDBMigration(db); err != nil {
		return fmt.Errorf("could not complete db migration: %w", err)
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

		"db.dbname":   "watcher",
		"db.host":     "localhost",
		"db.port":     5432,
		"db.password": "watcher",
		"db.user":     "watcher",

		"valkey.host": "localhost",
		"valkey.port": 6379,
	}, "."), nil)
}
