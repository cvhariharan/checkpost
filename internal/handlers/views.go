package handlers

import (
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleMachines(c echo.Context) error {
	machines := []Machine{
		{ID: "m1", Hostname: "web-server-01", Platform: "ubuntu 22.04", Status: "active", LastSeen: "2 minutes ago", Tags: []string{"production", "web", "endpoint", "tech"}},
		{ID: "m2", Hostname: "db-server-01", Platform: "debian 11", Status: "active", LastSeen: "5 minutes ago", Tags: []string{"production", "database", "mobile"}},
		{ID: "m3", Hostname: "worker-01", Platform: "centos 8", Status: "offline", LastSeen: "2 days ago", Tags: []string{"staging", "worker"}},
		{ID: "m4", Hostname: "cache-01", Platform: "ubuntu 20.04", Status: "active", LastSeen: "1 minute ago", Tags: []string{"production", "cache"}},
	}

	// Get unique tags from machines
	allTags := make(map[string]bool)
	for _, m := range machines {
		for _, t := range m.Tags {
			allTags[t] = true
		}
	}
	uniqueTags := make([]string, 0, len(allTags))
	for t := range allTags {
		uniqueTags = append(uniqueTags, t)
	}

	return c.Render(200, "base.html", IndexPageData{
		Title:    "Machines",
		Active:   "machines",
		Machines: machines,
		AllTags:  uniqueTags,
	})
}

func (h *Handler) HandleQueries(c echo.Context) error {
	return c.Render(200, "base.html", IndexPageData{
		Title:  "Queries",
		Active: "queries",
	})
}

func (h *Handler) HandleSchedules(c echo.Context) error {
	schedules := []Schedule{
		{ID: "s1", Name: "Daily Security Scan", Query: "Security Essentials", Interval: "24h", LastRun: "12 hours ago", NextRun: "in 12 hours"},
		{ID: "s2", Name: "Hourly Performance Check", Query: "Performance Monitoring", Interval: "1h", LastRun: "45 minutes ago", NextRun: "in 15 minutes"},
		{ID: "s3", Name: "Weekly Compliance", Query: "Compliance Checks", Interval: "168h", LastRun: "3 days ago", NextRun: "in 4 days"},
		{ID: "s4", Name: "Network Status", Query: "Network Analysis", Interval: "6h", LastRun: "2 hours ago", NextRun: "in 4 hours"},
	}
	return c.Render(200, "base.html", IndexPageData{
		Title:     "Schedules",
		Active:    "schedules",
		Schedules: schedules,
	})
}
