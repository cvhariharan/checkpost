package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"errors"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/handlers"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/labstack/echo/v4"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var k = koanf.New(".")

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config.toml", "Path to the config file. If the file doesn't exist, a default file will be generated")
	flag.Parse()

	if err := setDefaultConfig(); err != nil {
		log.Fatalf("error creating default config: %v", err)
	}

	if err := loadOrCreateConfigFile(configFile); err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	if err := runDBMigration(); err != nil {
		log.Fatalf("could not perform db migration: %v", err)
	}

	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		log.Fatal(err)
	}

	e := echo.New()

	h := handlers.NewHandler()
	e.Renderer = handlers.NewTemplateRenderer()
	e.HTTPErrorHandler = handlers.ErrorHandler

	e.Static("/static", "web/static")

	e.GET("/", func(c echo.Context) error {
		return c.Redirect(301, "/machines")
	})

	e.GET("/machines", h.HandleMachines)
	e.GET("/queries", h.HandleQueries)
	e.GET("/packs", h.HandlePacks)
	e.GET("/schedules", h.HandleSchedules)

	e.Logger.Fatal(e.Start(":1323"))
}

func runDBMigration() error {
	db, err := sqlx.Connect("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", k.String("db.user"), k.String("db.password"), k.String("db.host"), k.Int("db.port"), k.String("db.dbname")))
	if err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		return fmt.Errorf("could not complete db migration: %w", err)
	}

	return nil
}


func initDB(db *sqlx.DB) error {
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
