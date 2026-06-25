package core

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/cvhariharan/checkpost/internal/config"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/cvhariharan/checkpost/internal/results"
)

var (
	ErrInvalidLogType         = errors.New("invalid log type")
	ErrInvalidNodeDisplayName = errors.New("invalid node display name")
	ErrInvalidQuery           = errors.New("invalid query")
	ErrResultsBackendDisabled = errors.New("results backend not configured")
)

type Core struct {
	store         repo.Store
	logger        *slog.Logger
	rootURL       string
	adminUsername string
	enforcer      *casbin.Enforcer

	sink   results.Sink
	reader results.Reader

	systemMetrics *SystemMetricsRegistry

	policyUpdateInterval time.Duration
	policyStaleAfter     time.Duration
}

func NewCore(logger *slog.Logger, store repo.Store, sink results.Sink, reader results.Reader, enforcer *casbin.Enforcer, cfg config.AppConfig) (*Core, error) {
	return &Core{
		store:                store,
		logger:               logger.WithGroup("core"),
		rootURL:              cfg.RootURL,
		adminUsername:        cfg.AdminUsername,
		enforcer:             enforcer,
		sink:                 sink,
		reader:               reader,
		systemMetrics:        DefaultSystemMetrics(),
		policyUpdateInterval: cfg.PolicyUpdateInterval,
		policyStaleAfter:     cfg.PolicyStaleAfter,
	}, nil
}

func postgresInterval(value time.Duration) string {
	seconds := int64(value / time.Second)
	if seconds <= 0 {
		seconds = 1
	}
	return fmt.Sprintf("%d seconds", seconds)
}
