package shipper

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/redis/go-redis/v9"
)

const (
	ResultsStream = "log:result"
	StatusStream  = "log:status"
	Workers       = 4
)

type Shipper struct {
	logger *slog.Logger
	name string
	conn driver.Conn
	r    redis.UniversalClient
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
		name: name,
		conn: conn,
		r:    r,
	}, nil
}

func (s *Shipper) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	for range Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.process(ctx); err != nil {
				fmt.Printf("error processing: %v\n", err)
			}
		}()
	}

	wg.Wait()

	return nil
}

type OsqueryLog struct {
	Action         string          `json:"action"`
	CalendarTime   string          `json:"calendarTime"`
	Counter        uint64          `json:"counter"`
	Epoch          uint64          `json:"epoch"`
	HostIdentifier string          `json:"hostIdentifier"`
	Name           string          `json:"name"`
	Numerics       bool            `json:"numerics"`
	UnixTime       int64           `json:"unixTime"`
	Columns        json.RawMessage `json:"columns"`
}

func (s *Shipper) process(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			resultsEntries, err := s.r.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    fmt.Sprintf("%s-results", s.name),
				Consumer: fmt.Sprintf("%s-consumer", s.name),
				Streams:  []string{ResultsStream, ">"},
				Count:    10,
				Block:    time.Second * 1,
			}).Result()
			if err != nil && err != redis.Nil {
				s.logger.Error("error reading entries from stream", "error", err)
				return err
			}

			if len(resultsEntries) > 0 && len(resultsEntries[0].Messages) > 0 {
				batch := make([]OsqueryLog, 0, len(resultsEntries[0].Messages))
				ackIDs := make([]string, 0, len(resultsEntries[0].Messages))

				for _, msg := range resultsEntries[0].Messages {
					var log OsqueryLog
					if msg.Values["msg"] == nil {
						continue
					}
					if err := json.Unmarshal([]byte(msg.Values["msg"].(string)), &log); err != nil {
						fmt.Printf("Error unmarshaling result log: %v\n", err)
						continue
					}
					batch = append(batch, log)
					ackIDs = append(ackIDs, msg.ID)
				}

				if len(batch) > 0 {
					if err := s.insertResults(ctx, batch); err != nil {
						s.logger.Error("error inserting results", "error", err)
						return fmt.Errorf("failed to insert results: %w", err)
					}

					if err := s.r.XAck(ctx, ResultsStream, fmt.Sprintf("%s-results", s.name), ackIDs...).Err(); err != nil {
						s.logger.Error("error ack results", "error", err)
						return fmt.Errorf("failed to acknowledge results: %w", err)
					}
				}
			}

			statusEntries, err := s.r.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    fmt.Sprintf("%s-status", s.name),
				Consumer: fmt.Sprintf("%s-consumer", s.name),
				Streams:  []string{StatusStream, ">"},
				Count:    10,
				Block:    time.Second * 1,
			}).Result()
			if err != nil && err != redis.Nil {
				s.logger.Error("error reading entries from stream", "error", err)
				return err
			}

			if len(statusEntries) > 0 && len(statusEntries[0].Messages) > 0 {
				batch := make([]OsqueryLog, 0, len(statusEntries[0].Messages))
				ackIDs := make([]string, 0, len(statusEntries[0].Messages))

				for _, msg := range statusEntries[0].Messages {
					var log OsqueryLog
					if msg.Values["msg"] == nil {
						continue
					}
					if err := json.Unmarshal([]byte(msg.Values["msg"].(string)), &log); err != nil {
						fmt.Printf("Error unmarshaling status log: %v\n", err)
						continue
					}
					batch = append(batch, log)
					ackIDs = append(ackIDs, msg.ID)
				}

				if len(batch) > 0 {
					if err := s.insertStatus(ctx, batch); err != nil {
						s.logger.Error("error inserting status", "error", err)
						return fmt.Errorf("failed to insert status: %w", err)
					}

					// Acknowledge processed messages
					if err := s.r.XAck(ctx, StatusStream, fmt.Sprintf("%s-status", s.name), ackIDs...).Err(); err != nil {
						s.logger.Error("error ack status", "error", err)
						return fmt.Errorf("failed to acknowledge status: %w", err)
					}
				}
			}
		}
	}
}

func (s *Shipper) insertResults(ctx context.Context, logs []OsqueryLog) error {
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO osquery_results")
	if err != nil {
		return err
	}

	for _, log := range logs {
		s.logger.Debug("log entry", "columns", string(log.Columns))
		unixTime := time.Unix(log.UnixTime, 0)
		if err := batch.Append(
			log.Action,
			log.CalendarTime,
			log.Counter,
			log.Epoch,
			log.HostIdentifier,
			log.Name,
			log.Numerics,
			unixTime,
			log.Columns,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func (s *Shipper) insertStatus(ctx context.Context, logs []OsqueryLog) error {
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO osquery_status")
	if err != nil {
		return err
	}

	for _, log := range logs {
		unixTime := time.Unix(log.UnixTime, 0)
		if err := batch.Append(
			log.Action,
			log.CalendarTime,
			log.Counter,
			log.Epoch,
			log.HostIdentifier,
			log.Name,
			log.Numerics,
			unixTime,
			log.Columns,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}
