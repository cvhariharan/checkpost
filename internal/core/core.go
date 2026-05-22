package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
)

var (
	ErrInvalidLogType = errors.New("invalid log type")
	ErrInvalidQuery   = errors.New("invalid query")
)

type Core struct {
	store  repo.Store
	logger *slog.Logger

	systemSchedulesMap map[string]bool

	adHocMu           sync.Mutex
	adHocPending      map[string][]models.MachineQueryResult
	adHocHistory      map[string][]models.MachineQueryResult
	adHocNodeKeyToID  map[string]string
	adHocWaiters      map[string]chan models.MachineQueryResult
	adHocQueryTimeout time.Duration

	policyUpdateInterval time.Duration
	policyStaleAfter     time.Duration
}

func NewCore(logger *slog.Logger, store repo.Store, cfg config.AppConfig) (*Core, error) {
	schedules, err := store.ListSystemScheduleNames(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load system schedules: %w", err)
	}

	policyUpdateInterval, err := parseDurationConfig(cfg.PolicyUpdateInterval, time.Hour)
	if err != nil {
		return nil, fmt.Errorf("parse policy update interval: %w", err)
	}
	policyStaleAfter, err := parseDurationConfig(cfg.PolicyStaleAfter, 2*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("parse policy stale after: %w", err)
	}

	m := make(map[string]bool, len(schedules))
	for _, sched := range schedules {
		m[sched] = true
	}

	return &Core{
		store:                store,
		logger:               logger.WithGroup("core"),
		systemSchedulesMap:   m,
		adHocPending:         make(map[string][]models.MachineQueryResult),
		adHocHistory:         make(map[string][]models.MachineQueryResult),
		adHocNodeKeyToID:     make(map[string]string),
		adHocWaiters:         make(map[string]chan models.MachineQueryResult),
		adHocQueryTimeout:    20 * time.Second,
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
