package parquet

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/zstd"
)

const (
	flushInterval      = 30 * time.Second
	maxFlushRows       = 5000
	partitionQueueSize = 1024
	overflowQueueSize  = 50000
)

type Writer struct {
	root   string
	store  SchemaStore
	logger *slog.Logger

	mu       sync.Mutex
	workers  map[PartitionKey]*partitionWorker
	deleting map[uuid.UUID]struct{}
	closed   bool

	overflow    chan submission
	overflowWG  sync.WaitGroup
	partitionWG sync.WaitGroup
}

type submission struct {
	key PartitionKey
	row results.Row
}

type partitionWorker struct {
	key  PartitionKey
	in   chan results.Row
	done chan struct{}
}

func NewWriter(root string, store SchemaStore, logger *slog.Logger) (*Writer, error) {
	if root == "" {
		return nil, errors.New("results: parquet root is empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create parquet root: %w", err)
	}
	w := &Writer{
		root:     root,
		store:    store,
		logger:   logger.WithGroup("results.writer"),
		workers:  make(map[PartitionKey]*partitionWorker),
		deleting: make(map[uuid.UUID]struct{}),
		overflow: make(chan submission, overflowQueueSize),
	}
	w.overflowWG.Add(1)
	go w.drainOverflow()
	return w, nil
}

func (w *Writer) Submit(sourceUUID uuid.UUID, sqlVersion int32, kind string, rows []results.Row) error {
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		key := PartitionKey{SourceUUID: sourceUUID, SQLVersion: sqlVersion, NodeID: row.NodeID, Kind: kind}
		w.mu.Lock()
		if w.closed {
			w.mu.Unlock()
			return ErrClosed
		}
		if _, gone := w.deleting[sourceUUID]; gone {
			w.mu.Unlock()
			// Schedule is being deleted; drop the row silently — callers
			// (ingest path) shouldn't see a hard error for a transient
			// race with a delete.
			return nil
		}
		worker := w.workerForLocked(key)
		select {
		case worker.in <- row:
		default:
			select {
			case w.overflow <- submission{key: key, row: row}:
			default:
				w.mu.Unlock()
				return results.ErrBackpressure
			}
		}
		w.mu.Unlock()
	}
	return nil
}

func (w *Writer) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	close(w.overflow)
	w.mu.Unlock()

	w.overflowWG.Wait()

	w.mu.Lock()
	for _, pw := range w.workers {
		close(pw.in)
	}
	w.mu.Unlock()
	w.partitionWG.Wait()
	return nil
}

// Delete drains in-flight writes, removes the source's partition workers, then
// deletes its parquet tree. It blocks until every partition goroutine has
// flushed, so no further chunks are created under the directory afterward.
func (w *Writer) Delete(sourceUUID uuid.UUID) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrClosed
	}
	w.deleting[sourceUUID] = struct{}{}

	dones := make([]chan struct{}, 0)
	for key, pw := range w.workers {
		if key.SourceUUID != sourceUUID {
			continue
		}
		close(pw.in)
		dones = append(dones, pw.done)
		delete(w.workers, key)
	}
	w.mu.Unlock()

	for _, d := range dones {
		<-d
	}

	dir := queryDir(w.root, sourceUUID)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove source dir: %w", err)
	}
	return nil
}

func (w *Writer) Root() string { return w.root }

func (w *Writer) workerForLocked(key PartitionKey) *partitionWorker {
	if pw, ok := w.workers[key]; ok {
		return pw
	}
	pw := &partitionWorker{
		key:  key,
		in:   make(chan results.Row, partitionQueueSize),
		done: make(chan struct{}),
	}
	w.workers[key] = pw
	w.partitionWG.Add(1)
	go w.runPartition(pw)
	return pw
}

func (w *Writer) drainOverflow() {
	defer w.overflowWG.Done()
	for s := range w.overflow {
		// Hold w.mu through the send so DeleteSchedule cannot close the
		// target channel between lookup and write. The partition worker
		// drains pw.in independently of w.mu, so the send completes even
		// under load.
		w.mu.Lock()
		if _, gone := w.deleting[s.key.SourceUUID]; gone {
			w.mu.Unlock()
			continue
		}
		worker := w.workerForLocked(s.key)
		worker.in <- s.row
		w.mu.Unlock()
	}
}

func (w *Writer) runPartition(pw *partitionWorker) {
	defer close(pw.done)
	defer w.partitionWG.Done()

	var buffer []results.Row
	timer := time.NewTimer(flushInterval)
	defer timer.Stop()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
		if err := w.writeChunk(pw.key, buffer); err != nil {
			w.logger.Error("write parquet chunk",
				"source_uuid", pw.key.SourceUUID,
				"sql_version", pw.key.SQLVersion,
				"node_id", pw.key.NodeID,
				"rows", len(buffer),
				"error", err)
		}
		buffer = buffer[:0]
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(flushInterval)
	}

	for {
		select {
		case row, ok := <-pw.in:
			if !ok {
				flush()
				return
			}
			buffer = append(buffer, row)
			if len(buffer) >= maxFlushRows {
				flush()
			}
		case <-timer.C:
			flush()
		}
	}
}

func (w *Writer) writeChunk(key PartitionKey, rows []results.Row) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	columns, err := w.resolveColumns(ctx, key, rows)
	if err != nil {
		return fmt.Errorf("resolve columns: %w", err)
	}

	dir := partitionDir(w.root, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create partition dir: %w", err)
	}

	name := fmt.Sprintf("chunk-%s.parquet", newULID())
	finalPath := filepath.Join(dir, name)
	tmpPath := finalPath + ".tmp"

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}

	if err := encodeRows(f, rows, columns); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode rows: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("fsync chunk: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close chunk: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename chunk: %w", err)
	}
	return nil
}

func (w *Writer) resolveColumns(ctx context.Context, key PartitionKey, rows []results.Row) ([]string, error) {
	seen := make(map[string]struct{})
	observed := make([]string, 0, 8)
	for _, row := range rows {
		for col := range row.Columns {
			if col == "" {
				continue
			}
			if _, ok := seen[col]; ok {
				continue
			}
			seen[col] = struct{}{}
			observed = append(observed, col)
		}
	}
	sort.Strings(observed)

	final, err := w.store.MergeColumns(ctx, key.SourceUUID, key.SQLVersion, observed, key.Kind)
	if err != nil {
		return nil, fmt.Errorf("merge schema: %w", err)
	}
	return final, nil
}

func encodeRows(f *os.File, rows []results.Row, queryColumns []string) error {
	schema, columnIndex := buildSchema(queryColumns)
	writer := parquet.NewWriter(f, schema, parquet.Compression(&zstd.Codec{}))

	ingestedAt := time.Now().UTC()
	prows := make([]parquet.Row, 0, len(rows))
	for _, row := range rows {
		prow := make(parquet.Row, len(columnIndex))
		setVal := func(name string, v parquet.Value) {
			idx, ok := columnIndex[name]
			if !ok {
				return
			}
			prow[idx] = v.Level(0, 0, idx)
		}
		setVal(ColNodeID, parquet.Int64Value(row.NodeID))
		setVal(ColUnixTime, parquet.Int64Value(row.UnixTime.UTC().UnixMicro()))
		setVal(ColCalendarTime, parquet.ByteArrayValue([]byte(row.CalendarTime)))
		setVal(ColAction, parquet.ByteArrayValue([]byte(row.Action)))
		setVal(ColRowHash, parquet.ByteArrayValue(row.RowHash))
		setVal(ColIngestedAt, parquet.Int64Value(ingestedAt.UnixMicro()))

		for _, col := range queryColumns {
			idx := columnIndex[col]
			if v, ok := row.Columns[col]; ok {
				prow[idx] = parquet.ByteArrayValue([]byte(v)).Level(0, 1, idx)
			} else {
				prow[idx] = parquet.NullValue().Level(0, 0, idx)
			}
		}

		prows = append(prows, prow)
	}
	if _, err := writer.WriteRows(prows); err != nil {
		return err
	}
	return writer.Close()
}

func buildSchema(queryColumns []string) (*parquet.Schema, map[string]int) {
	group := parquet.Group{
		ColNodeID:       parquet.Int(64),
		ColUnixTime:     parquet.Timestamp(parquet.Microsecond),
		ColCalendarTime: parquet.String(),
		ColAction:       parquet.String(),
		ColRowHash:      parquet.Leaf(parquet.ByteArrayType),
		ColIngestedAt:   parquet.Timestamp(parquet.Microsecond),
	}
	for _, col := range queryColumns {
		group[col] = parquet.Optional(parquet.String())
	}

	schema := parquet.NewSchema("result", group)

	idx := make(map[string]int, len(group))
	for i, c := range schema.Columns() {
		idx[c[len(c)-1]] = i
	}
	return schema, idx
}

var ulidEntropy = ulid.Monotonic(rand.Reader, 0)
var ulidMu sync.Mutex

var ErrClosed = errors.New("results: writer closed")

func newULID() string {
	ulidMu.Lock()
	defer ulidMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulidEntropy).String()
}
