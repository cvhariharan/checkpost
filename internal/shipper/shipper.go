package shipper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

const (
	ResultsStream = "log:result"
	StatusStream  = "log:status"

	// Workers is the maximum worker threads that will be used for processing results and status.
	// This will never exceed the max, it may be less than the actual value. The total count is divided
	// in half when workers are created. Each worker internal creates 2 workers to process results and status.
	Workers = 4
)

type Shipper struct {
	logger *slog.Logger
	name   string
	conn   driver.Conn
	r      redis.UniversalClient
}

func NewShipper(logger *slog.Logger, name string, conn driver.Conn, r redis.UniversalClient) (*Shipper, error) {
	if err := r.XGroupCreate(context.Background(), ResultsStream, fmt.Sprintf("%s-results", name), "0").Err(); err != nil {
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return nil, err
		}
	}

	if err := r.XGroupCreate(context.Background(), StatusStream, fmt.Sprintf("%s-status", name), "0").Err(); err != nil {
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return nil, err
		}
	}

	return &Shipper{
		logger: logger.WithGroup("shipper"),
		name:   name,
		conn:   conn,
		r:      r,
	}, nil
}

func (s *Shipper) Run(ctx context.Context) error {
	var eg errgroup.Group

	if Workers < 2 {
		return fmt.Errorf("expected atleast 2 workers, got %d", Workers)
	}

	for range Workers / 2 {
		eg.Go(func() error {
			if err := s.process(ctx); err != nil {
				return fmt.Errorf("error processing logs: %w", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func (s *Shipper) process(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var eg errgroup.Group
			eg.Go(func() error {
				if err := s.processResults(ctx); err != nil {
					return fmt.Errorf("error processing results: %w", err)
				}
				return nil
			})

			eg.Go(func() error {
				if err := s.processStatus(ctx); err != nil {
					return fmt.Errorf("error processing status: %w", err)
				}
				return nil
			})

			if err := eg.Wait(); err != nil {
				return err
			}
		}
	}
}

type ResultEntry struct {
	Action         string              `json:"action"`
	CalendarTime   string              `json:"calendarTime"`
	Counter        uint64              `json:"counter"`
	Epoch          uint64              `json:"epoch"`
	HostIdentifier string              `json:"hostIdentifier"`
	Name           string              `json:"name"`
	Numerics       bool                `json:"numerics"`
	UnixTime       int64               `json:"unixTime"`
	Columns        []map[string]string `json:"columns"`
	Snapshot       []map[string]string `json:"snapshot"`
}

type Results struct {
	LogType string        `json:"log_type"`
	Data    []ResultEntry `json:"data"`
}

type StatusEntry struct {
	CalendarTime   string `json:"calendarTime"`
	FileName       string `json:"filename"`
	HostIdentifier string `json:"hostIdentifier"`
	Line           uint32 `json:"line"`
	Message        string `json:"message"`
	Severity       uint   `json:"severity"`
	UnixTime       int64  `json:"unixTime"`
	Version        string `json:"version"`
}

type Statuses struct {
	LogType string        `json:"log_type"`
	Data    []StatusEntry `json:"data"`
}

type LogEntry interface {
	ResultEntry | StatusEntry
}

type LogData[T LogEntry] interface {
	*Results | *Statuses
	GetData() []T
}

func (r *Results) GetData() []ResultEntry {
	return r.Data
}

func (s *Statuses) GetData() []StatusEntry {
	return s.Data
}

// processEntries is a generic function that processes entries from a Redis stream
func processEntries[T LogEntry, D LogData[T]](s *Shipper, ctx context.Context, stream string, groupSuffix string, newLogData func() D, insertFunc func(context.Context, []T) error) error {
	entries, err := s.r.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    fmt.Sprintf("%s-%s", s.name, groupSuffix),
		Consumer: fmt.Sprintf("%s-consumer", s.name),
		Streams:  []string{stream, ">"},
		Count:    10,
		Block:    time.Second * 1,
	}).Result()
	if err != nil && err != redis.Nil {
		s.logger.Error("error reading entries from stream", "error", err)
		return err
	}

	if len(entries) > 0 && len(entries[0].Messages) > 0 {
		batch := make([]T, 0, len(entries[0].Messages))
		ackIDs := make([]string, 0, len(entries[0].Messages))

		for _, msg := range entries[0].Messages {
			logData := newLogData()
			if msg.Values["msg"] == nil {
				continue
			}
			if err := json.Unmarshal([]byte(msg.Values["msg"].(string)), logData); err != nil {
				fmt.Printf("Error unmarshaling log: %v\n", err)
				continue
			}
			batch = append(batch, logData.GetData()...)
			ackIDs = append(ackIDs, msg.ID)
		}

		if len(batch) > 0 {
			s.logger.Debug("batch", "batch", batch)
			if err := insertFunc(ctx, batch); err != nil {
				return fmt.Errorf("failed to insert data: %w", err)
			}

			if err := s.r.XAck(ctx, stream, fmt.Sprintf("%s-%s", s.name, groupSuffix), ackIDs...).Err(); err != nil {
				return fmt.Errorf("failed to acknowledge data: %w", err)
			}
		}
	}

	return nil
}

func (s *Shipper) processResults(ctx context.Context) error {
	return processEntries(
		s,
		ctx,
		ResultsStream,
		"results",
		func() *Results { return &Results{} },
		s.insertResults,
	)
}

func (s *Shipper) processStatus(ctx context.Context) error {
	return processEntries(
		s,
		ctx,
		StatusStream,
		"status",
		func() *Statuses { return &Statuses{} },
		s.insertStatus,
	)
}

func (s *Shipper) insertResults(ctx context.Context, logs []ResultEntry) error {
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO osquery_results")
	if err != nil {
		return err
	}

	for _, log := range logs {
		unixTime := time.Unix(log.UnixTime, 0)

		var colsData []map[string]string
		if log.Action == "snapshot" {
			colsData = log.Snapshot
		} else {
			colsData = log.Columns
		}

		for _, col := range colsData {
			c, err := json.Marshal(col)
			if err != nil {
				return err
			}

			if err := batch.Append(
				log.Action,
				log.CalendarTime,
				log.Counter,
				log.Epoch,
				log.HostIdentifier,
				log.Name,
				log.Numerics,
				unixTime,
				c,
			); err != nil {
				return err
			}
		}
	}

	return batch.Send()
}

func (s *Shipper) insertStatus(ctx context.Context, logs []StatusEntry) error {
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO osquery_status")
	if err != nil {
		return err
	}

	for _, log := range logs {
		s.logger.Debug("received json status", "body", log)
		unixTime := time.Unix(log.UnixTime, 0)
		if err := batch.Append(
			log.CalendarTime,
			log.FileName,
			log.HostIdentifier,
			log.Line,
			log.Message,
			unixTime,
			log.Severity,
			log.Version,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}
