package results

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

type boundSink struct {
	sink     Sink
	required bool
}

// MultiSink fans a batch out to every configured backend. An empty MultiSink
// is a no-op.
type MultiSink struct {
	sinks  []boundSink
	logger *slog.Logger
}

func NewMultiSink(logger *slog.Logger) *MultiSink {
	return &MultiSink{logger: logger.WithGroup("results.multisink")}
}

// Add registers a backend. A required backend's Submit error propagates to the
// caller; best-effort errors are logged and absorbed.
func (m *MultiSink) Add(sink Sink, required bool) {
	m.sinks = append(m.sinks, boundSink{sink: sink, required: required})
}

func (m *MultiSink) Name() string { return "multi" }

// Len reports how many backends are registered.
func (m *MultiSink) Len() int { return len(m.sinks) }

func (m *MultiSink) Submit(ctx context.Context, batch Batch) error {
	var backpressure bool
	var reqErr error
	for _, bs := range m.sinks {
		err := bs.sink.Submit(ctx, batch)
		if err == nil {
			continue
		}
		if !bs.required {
			m.logger.Warn("best-effort sink submit failed", "sink", bs.sink.Name(), "error", err)
			continue
		}
		if errors.Is(err, ErrBackpressure) {
			backpressure = true
		} else if reqErr == nil {
			reqErr = err
		}
	}
	if backpressure {
		return ErrBackpressure
	}
	return reqErr
}

// Flush fans the request out to every backend that implements Flusher. A
// required backend's flush error propagates; best-effort errors are logged and
// absorbed. Backends without a buffer are skipped.
func (m *MultiSink) Flush(ctx context.Context, sourceUUID uuid.UUID) error {
	var reqErr error
	for _, bs := range m.sinks {
		f, ok := bs.sink.(Flusher)
		if !ok {
			continue
		}
		if err := f.Flush(ctx, sourceUUID); err != nil {
			if !bs.required {
				m.logger.Warn("best-effort sink flush failed", "sink", bs.sink.Name(), "error", err)
				continue
			}
			if reqErr == nil {
				reqErr = err
			}
		}
	}
	return reqErr
}

func (m *MultiSink) Delete(ctx context.Context, sourceUUID uuid.UUID) error {
	for _, bs := range m.sinks {
		if err := bs.sink.Delete(ctx, sourceUUID); err != nil {
			m.logger.Error("sink delete failed", "sink", bs.sink.Name(), "source_uuid", sourceUUID, "error", err)
		}
	}
	return nil
}

func (m *MultiSink) Close() error {
	var errs []error
	for _, bs := range m.sinks {
		if err := bs.sink.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", bs.sink.Name(), err))
		}
	}
	return errors.Join(errs...)
}
