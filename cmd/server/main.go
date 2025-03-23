package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/handlers"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/labstack/echo/v4"
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
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("could not generate random key for securecookie encryption: %w", err)
	}
	return k.Load(confmap.Provider(map[string]any{
		"app.admin_username":    "watcher_admin",
		"app.admin_password":    "watcher_password",
		"app.http_tls_cert":     "server_cert.pem",
		"app.http_tls_key":      "server_key.pem",
		"app.root_url":          "http://localhost:1323",
		"app.secure_cookie_key": hex.EncodeToString(key),

		"db.dbname":   "watcher",
		"db.host":     "localhost",
		"db.port":     5432,
		"db.password": "watcher",
		"db.user":     "watcher",

		"valkey.host": "localhost",
		"valkey.port": 6379,
	}, "."), nil)
}
