package core

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/cvhariharan/watcher/internal/results"
)

var (
	ErrInvalidLogType = errors.New("invalid log type")
	ErrInvalidQuery   = errors.New("invalid query")
)

type Core struct {
	store  repo.Store
	logger *slog.Logger

	results       *results.Writer
	resultsReader *results.Reader

	systemMetrics *SystemMetricsRegistry

	policyUpdateInterval time.Duration
	policyStaleAfter     time.Duration
}

func NewCore(logger *slog.Logger, store repo.Store, writer *results.Writer, reader *results.Reader, cfg config.AppConfig) (*Core, error) {
	policyUpdateInterval, err := parseDurationConfig(cfg.PolicyUpdateInterval, time.Hour)
	if err != nil {
		return nil, fmt.Errorf("parse policy update interval: %w", err)
	}
	policyStaleAfter, err := parseDurationConfig(cfg.PolicyStaleAfter, 2*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("parse policy stale after: %w", err)
	}

	return &Core{
		store:                store,
		logger:               logger.WithGroup("core"),
		results:              writer,
		resultsReader:        reader,
		systemMetrics:        DefaultSystemMetrics(),
		policyUpdateInterval: policyUpdateInterval,
		policyStaleAfter:     policyStaleAfter,
	}, nil
}

func parseDurationConfig(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return parsed, nil
}

func postgresInterval(value time.Duration) string {
	seconds := int64(value / time.Second)
	if seconds <= 0 {
		seconds = 1
	}
	return fmt.Sprintf("%d seconds", seconds)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultBool(value, fallback bool) bool {
	if value {
		return true
	}
	return fallback
}

func pageCountFor(total, perPage int) int {
	if total == 0 || perPage <= 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}
