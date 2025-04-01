package logqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const (
	LogStreamPrefix = "log:%s"
)

type LogMsg struct {
	LogType string                   `json:"log_type"`
	Data    []map[string]interface{} `json:"data"`
}

type StreamLogger struct {
	logger *slog.Logger
	r      redis.UniversalClient
}

func NewStreamLogger(logger *slog.Logger, r redis.UniversalClient) *StreamLogger {
	return &StreamLogger{logger: logger, r: r}
}

func (s *StreamLogger) WriteLog(ctx context.Context, msg LogMsg) error {
	if msg.LogType == "" {
		return fmt.Errorf("log type cannot be empty")
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error marshalling log msg: %w", err)
	}

	return s.r.XAdd(ctx, &redis.XAddArgs{
		Stream: fmt.Sprintf(LogStreamPrefix, msg.LogType),
		Values: msgBytes,
	}).Err()
}
