package shipper

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// SystemShipper stores data in the watcher tables
type SystemShipper struct {
	logger *slog.Logger
	name   string
	db     *sqlx.DB
	r      redis.UniversalClient
}

func NewSystemShipper(logger *slog.Logger, name string, db *sqlx.DB, r redis.UniversalClient) (Shipper, error) {
	if err := r.XGroupCreate(context.Background(), ResultsStream, fmt.Sprintf("%s-results", name), "0").Err(); err != nil {
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return nil, err
		}
	}

	return &SystemShipper{
		logger: logger.WithGroup("shipper"),
		name:   name,
		db:     db,
		r:      r,
	}, nil
}

func (s *SystemShipper) Run(ctx context.Context) error {
	return nil
}

// func (s *SystemShipper) process(ctx context.Context) error {
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		default:
// 			entries, err := s.r.XReadGroup(ctx, &redis.XReadGroupArgs{
// 				Group:    fmt.Sprintf("%s-results", s.name),
// 				Consumer: fmt.Sprintf("%s-consumer", s.name),
// 				Streams:  []string{ResultsStream, ">"},
// 				Count:    10,
// 				Block:    time.Second * 1,
// 			}).Result()

// 			if err != nil {
// 				return fmt.Errorf("error reading results in system shipper: %w", err)
// 			}

// 			if len(entries) > 0 && len(entries[0].Messages) > 0 {
// 				batch := make([]ResultEntry, 0, len(entries[0].Messages))
// 				ackIDs := make([]string, 0, len(entries[0].Messages))

// 				for _, msg := range entries[0].Messages {
// 					logData := new(Results)
// 					if msg.Values["msg"] == nil {
// 						continue
// 					}
// 					if err := json.Unmarshal([]byte(msg.Values["msg"].(string)), logData); err != nil {
// 						fmt.Printf("Error unmarshaling log: %v\n", err)
// 						continue
// 					}
// 					batch = append(batch, logData.GetData()...)
// 					ackIDs = append(ackIDs, msg.ID)
// 				}

// 				if len(batch) > 0 {
// 					s.logger.Debug("batch", "batch", batch)
// 					if err := insertIntoSystemDB(ctx, batch); err != nil {
// 						return fmt.Errorf("failed to insert data: %w", err)
// 					}

// 					if err := s.r.XAck(ctx, ResultsStream, fmt.Sprintf("%s-results", s.name), ackIDs...).Err(); err != nil {
// 						return fmt.Errorf("failed to acknowledge data: %w", err)
// 					}
// 				}
// 			}
// 		}
// 	}
// }
