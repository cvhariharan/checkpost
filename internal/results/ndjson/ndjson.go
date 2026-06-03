// Package ndjson implements a result Sink that writes rows as newline-delimited
// JSON to a file or stdout for an external log pipeline to tail.
package ndjson

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

const (
	queueSize     = 4096
	flushInterval = 5 * time.Second
)

type record struct {
	ScheduleUUID string            `json:"schedule_uuid"`
	ScheduleName string            `json:"schedule_name"`
	SQLVersion   int32             `json:"sql_version"`
	NodeID       int64             `json:"node_id"`
	UnixTime     time.Time         `json:"unix_time"`
	CalendarTime string            `json:"calendar_time"`
	Action       string            `json:"action"`
	RowHash      string            `json:"row_hash"`
	Columns      map[string]string `json:"columns"`
}

// Sink writes records to a buffered writer drained by a single goroutine, so
// Submit never blocks on I/O. On overflow records are dropped and counted.
type Sink struct {
	w       *bufio.Writer
	closer  io.Closer // nil for stdout
	in      chan record
	done    chan struct{}
	logger  *slog.Logger
	dropped atomic.Uint64
}

// New opens path ("stdout" or empty for stdout, otherwise a file appended to).
func New(path string, logger *slog.Logger) (*Sink, error) {
	f := os.Stdout
	var closer io.Closer
	if path != "" && path != "stdout" {
		var err error
		f, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open ndjson file: %w", err)
		}
		closer = f
	}
	s := &Sink{
		w:      bufio.NewWriter(f),
		closer: closer,
		in:     make(chan record, queueSize),
		done:   make(chan struct{}),
		logger: logger.WithGroup("results.ndjson"),
	}
	go s.run()
	return s, nil
}

func (s *Sink) Name() string { return "ndjson" }

func (s *Sink) Submit(ctx context.Context, batch results.Batch) error {
	for _, row := range batch.Rows {
		rec := record{
			ScheduleUUID: batch.ScheduleUUID.String(),
			ScheduleName: batch.ScheduleName,
			SQLVersion:   batch.SQLVersion,
			NodeID:       row.NodeID,
			UnixTime:     row.UnixTime.UTC(),
			CalendarTime: row.CalendarTime,
			Action:       row.Action,
			RowHash:      fmt.Sprintf("%x", row.RowHash),
			Columns:      row.Columns,
		}
		select {
		case s.in <- rec:
		default:
			s.dropped.Add(1)
		}
	}
	return nil
}

func (s *Sink) DeleteSchedule(ctx context.Context, scheduleUUID uuid.UUID) error {
	return nil // already shipped downstream; nothing to delete
}

func (s *Sink) Close() error {
	close(s.in)
	<-s.done
	return nil
}

func (s *Sink) run() {
	defer close(s.done)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	enc := json.NewEncoder(s.w)
	for {
		select {
		case rec, ok := <-s.in:
			if !ok {
				s.flush()
				if s.closer != nil {
					s.closer.Close()
				}
				return
			}
			if err := enc.Encode(rec); err != nil {
				s.logger.Error("encode record", "error", err)
			}
		case <-ticker.C:
			s.flush()
			if n := s.dropped.Swap(0); n > 0 {
				s.logger.Warn("dropped records on overflow", "count", n)
			}
		}
	}
}

func (s *Sink) flush() {
	if err := s.w.Flush(); err != nil {
		s.logger.Error("flush", "error", err)
	}
}
