package main

import (
	"html/template"
	"io"
	"log"

	"github.com/labstack/echo/v4"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	err := t.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Println(err)
	}
	return err
}

type Machine struct {
	ID       string
	Hostname string
	Platform string
	Status   string
	LastSeen string
}

type Query struct {
	ID          string
	Name        string
	Description string
	SQL         string
	LastRun     string
}

type Pack struct {
	ID          string
	Name        string
	Description string
	Queries     int
	Targets     int
}

type Schedule struct {
	ID       string
	Name     string
	Query    string
	Interval string
	LastRun  string
	NextRun  string
}

type PageData struct {
	Title     string
	Active    string
	Machines  []Machine
	Queries   []Query
	Packs     []Pack
	Schedules []Schedule
}

func main() {
	e := echo.New()

	// Initialize templates with base layout
	t := &Template{
		templates: template.Must(template.ParseGlob("web/layouts/*.html")),
	}
	// Load all page templates in pages subdirectories
	template.Must(t.templates.ParseGlob("web/pages/**/*.html"))
	e.Renderer = t

	// Serve static files
	e.Static("/static", "web/static")

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(301, "/machines")
	})

	e.GET("/machines", func(c echo.Context) error {
		machines := []Machine{
			{ID: "m1", Hostname: "web-server-01", Platform: "ubuntu 22.04", Status: "active", LastSeen: "2 minutes ago"},
			{ID: "m2", Hostname: "db-server-01", Platform: "debian 11", Status: "active", LastSeen: "5 minutes ago"},
			{ID: "m3", Hostname: "worker-01", Platform: "centos 8", Status: "offline", LastSeen: "2 days ago"},
			{ID: "m4", Hostname: "cache-01", Platform: "ubuntu 20.04", Status: "active", LastSeen: "1 minute ago"},
		}
		return c.Render(200, "base.html", PageData{
			Title:    "Machines",
			Active:   "machines",
			Machines: machines,
		})
	})

	e.GET("/machines/query", func(c echo.Context) error {
		return c.Render(200, "base.html", PageData{
			Title:  "Query Machine",
			Active: "machines",
		})
	})

	e.GET("/queries", func(c echo.Context) error {
		queries := []Query{
			{ID: "q1", Name: "Running Processes", Description: "Lists all running processes", SQL: "SELECT * FROM processes", LastRun: "1 hour ago"},
			{ID: "q2", Name: "Open Ports", Description: "Shows open network ports", SQL: "SELECT * FROM listening_ports", LastRun: "30 minutes ago"},
			{ID: "q3", Name: "User Accounts", Description: "Lists user accounts", SQL: "SELECT * FROM users", LastRun: "2 hours ago"},
			{ID: "q4", Name: "System Info", Description: "Basic system information", SQL: "SELECT * FROM system_info", LastRun: "15 minutes ago"},
		}
		return c.Render(200, "base.html", PageData{
			Title:   "Queries",
			Active:  "queries",
			Queries: queries,
		})
	})

	e.GET("/packs", func(c echo.Context) error {
		packs := []Pack{
			{ID: "p1", Name: "Security Essentials", Description: "Basic security checks", Queries: 5, Targets: 10},
			{ID: "p2", Name: "Performance Monitoring", Description: "System performance metrics", Queries: 3, Targets: 8},
			{ID: "p3", Name: "Compliance Checks", Description: "Compliance related queries", Queries: 7, Targets: 15},
			{ID: "p4", Name: "Network Analysis", Description: "Network related checks", Queries: 4, Targets: 12},
		}
		return c.Render(200, "base.html", PageData{
			Title:  "Packs",
			Active: "packs",
			Packs:  packs,
		})
	})

	e.GET("/schedules", func(c echo.Context) error {
		schedules := []Schedule{
			{ID: "s1", Name: "Daily Security Scan", Query: "Security Essentials", Interval: "24h", LastRun: "12 hours ago", NextRun: "in 12 hours"},
			{ID: "s2", Name: "Hourly Performance Check", Query: "Performance Monitoring", Interval: "1h", LastRun: "45 minutes ago", NextRun: "in 15 minutes"},
			{ID: "s3", Name: "Weekly Compliance", Query: "Compliance Checks", Interval: "168h", LastRun: "3 days ago", NextRun: "in 4 days"},
			{ID: "s4", Name: "Network Status", Query: "Network Analysis", Interval: "6h", LastRun: "2 hours ago", NextRun: "in 4 hours"},
		}
		return c.Render(200, "base.html", PageData{
			Title:     "Schedules",
			Active:    "schedules",
			Schedules: schedules,
		})
	})

	e.Logger.Fatal(e.Start(":1323"))
}
