package parquet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
)

// Background worker cadences. Per the spec these are not configurable.
const (
	compactInterval   = 10 * time.Minute
	retentionInterval = 30 * time.Minute
	schemaGCInterval  = 24 * time.Hour
	compactThreshold  = 16
)

// Workers runs the periodic maintenance jobs over the parquet tree:
// chunk compaction, retention enforcement, and query_schemas garbage
// collection.
type Workers struct {
	root   string
	store  repo.Querier
	reader *Reader
	logger *slog.Logger

	mu     sync.Mutex
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewWorkers constructs the maintenance workers. Call Start to run them and
// Close to stop and wait for completion.
func NewWorkers(root string, store repo.Querier, reader *Reader, logger *slog.Logger) *Workers {
	return &Workers{
		root:   root,
		store:  store,
		reader: reader,
		logger: logger.WithGroup("results.workers"),
	}
}

// Start launches the background goroutines. It is a no-op if Start was
// already called and the workers are running.
func (w *Workers) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	w.wg.Add(3)
	go w.run(ctx, "compactor", compactInterval, w.compact)
	go w.run(ctx, "retention", retentionInterval, w.sweep)
	go w.run(ctx, "schema-gc", schemaGCInterval, w.schemaGC)
}

// Close stops the workers and waits for in-flight cycles to finish.
func (w *Workers) Close() error {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	w.wg.Wait()
	return nil
}

func (w *Workers) run(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) {
	defer w.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				w.logger.Error("worker cycle failed", "worker", name, "error", err)
			}
		}
	}
}

// compact walks each host directory and merges chunks once the count crosses
// compactThreshold. The merge uses DuckDB COPY with zstd; the result is
// written to a temp filename, fsynced, atomic-renamed, then the source
// chunks are deleted.
func (w *Workers) compact(ctx context.Context) error {
	hostDirs, err := listHostDirs(w.root)
	if err != nil {
		return fmt.Errorf("list host dirs: %w", err)
	}

	for _, dir := range hostDirs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		chunks, err := chunkFiles(dir)
		if err != nil {
			w.logger.Error("list chunks", "dir", dir, "error", err)
			continue
		}
		if len(chunks) < compactThreshold {
			continue
		}
		if err := w.mergeChunks(ctx, dir, chunks); err != nil {
			w.logger.Error("merge chunks", "dir", dir, "error", err)
		}
	}
	return nil
}

func (w *Workers) mergeChunks(ctx context.Context, dir string, chunks []string) error {
	if len(chunks) == 0 {
		return nil
	}
	merged := filepath.Join(dir, fmt.Sprintf("chunk-%s.parquet", newULID()))
	tmp := merged + ".tmp"

	// Build an explicit file list rather than a wildcard glob: a wildcard is
	// expanded inside DuckDB at COPY start, which would include any chunk a
	// partition worker flushes concurrently and then leave it on disk as a
	// duplicate of rows we've already merged.
	quoted := make([]string, 0, len(chunks))
	for _, c := range chunks {
		quoted = append(quoted, "'"+escapeSQLLiteral(c)+"'")
	}
	fileList := "[" + strings.Join(quoted, ", ") + "]"

	stmt := fmt.Sprintf(
		"COPY (SELECT * FROM read_parquet(%s, union_by_name=true) ORDER BY unix_time) TO '%s' (FORMAT PARQUET, COMPRESSION ZSTD)",
		fileList, escapeSQLLiteral(tmp),
	)
	if _, err := w.reader.db.ExecContext(ctx, stmt); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("duckdb copy: %w", err)
	}

	if err := fsyncFile(tmp); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("fsync merged: %w", err)
	}

	if err := os.Rename(tmp, merged); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename merged: %w", err)
	}
	if err := fsyncDir(dir); err != nil {
		w.logger.Warn("fsync partition dir", "dir", dir, "error", err)
	}
	for _, c := range chunks {
		if err := os.Remove(c); err != nil {
			w.logger.Error("remove source chunk", "chunk", c, "error", err)
		}
	}
	return nil
}

func fsyncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func fsyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := d.Sync()
	closeErr := d.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

// sweep deletes parquet files whose newest unix_time is older than the
// schedule's retention_days. Files are inspected with a small footer-only
// read via DuckDB.
func (w *Workers) sweep(ctx context.Context) error {
	hostDirs, err := listHostDirs(w.root)
	if err != nil {
		return fmt.Errorf("list host dirs: %w", err)
	}
	retentions, err := w.scheduleRetentions(ctx)
	if err != nil {
		return fmt.Errorf("load retentions: %w", err)
	}

	for _, dir := range hostDirs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		scheduleUUID, _, err := parseHostDir(dir, w.root)
		if err != nil {
			continue
		}
		days, ok := retentions[scheduleUUID]
		if !ok {
			days = 30
		}
		cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

		chunks, err := chunkFiles(dir)
		if err != nil {
			w.logger.Error("list chunks", "dir", dir, "error", err)
			continue
		}
		for _, chunk := range chunks {
			maxTime, err := w.maxUnixTime(ctx, chunk)
			if err != nil {
				w.logger.Error("read max unix_time", "chunk", chunk, "error", err)
				continue
			}
			if maxTime.Before(cutoff) {
				if err := os.Remove(chunk); err != nil {
					w.logger.Error("remove expired chunk", "chunk", chunk, "error", err)
				}
			}
		}
	}
	return nil
}

func (w *Workers) maxUnixTime(ctx context.Context, chunk string) (time.Time, error) {
	stmt := fmt.Sprintf("SELECT max(unix_time) FROM read_parquet('%s')", escapeSQLLiteral(chunk))
	var t sql.NullTime
	if err := w.reader.db.QueryRowContext(ctx, stmt).Scan(&t); err != nil {
		return time.Time{}, err
	}
	if !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, nil
}

// schemaGC removes query_schemas rows for (schedule_uuid, sql_version) pairs
// with no surviving parquet files.
func (w *Workers) schemaGC(ctx context.Context) error {
	schemas, err := w.store.ListAllQuerySchemas(ctx)
	if err != nil {
		return fmt.Errorf("list query_schemas: %w", err)
	}
	for _, s := range schemas {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		dir := versionDir(w.root, s.ScheduleUuid, s.SqlVersion)
		hasFiles, err := versionHasFiles(dir)
		if err != nil {
			w.logger.Error("scan version dir", "dir", dir, "error", err)
			continue
		}
		if hasFiles {
			continue
		}
		err = w.store.DeleteQuerySchema(ctx, repo.DeleteQuerySchemaParams{
			ScheduleUuid: s.ScheduleUuid,
			SqlVersion:   s.SqlVersion,
		})
		if err != nil {
			w.logger.Error("delete query_schema", "schedule_uuid", s.ScheduleUuid, "sql_version", s.SqlVersion, "error", err)
		}
	}
	return nil
}

// scheduleRetentions reads retention_days for every schedule. The result is a
// {schedule_uuid -> days} map; missing entries default to 30.
func (w *Workers) scheduleRetentions(ctx context.Context) (map[uuid.UUID]int32, error) {
	rows, err := w.store.ListScheduleRetentions(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int32, len(rows))
	for _, r := range rows {
		out[r.Uuid] = r.RetentionDays
	}
	return out, nil
}

// listHostDirs returns every directory at depth host=*/ under root.
func listHostDirs(root string) ([]string, error) {
	pattern := filepath.Join(root, "query=*", "v=*", "host=*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	dirs := matches[:0]
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || !info.IsDir() {
			continue
		}
		dirs = append(dirs, m)
	}
	return dirs, nil
}

// chunkFiles returns the sorted list of chunk-*.parquet files in dir,
// excluding any in-progress *.tmp temp files.
func chunkFiles(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "chunk-*.parquet"))
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// versionHasFiles reports whether any chunk parquet file exists under the
// given version directory across all hosts.
func versionHasFiles(versionDirPath string) (bool, error) {
	matches, err := filepath.Glob(filepath.Join(versionDirPath, "host=*", "chunk-*.parquet"))
	if err != nil {
		return false, err
	}
	return len(matches) > 0, nil
}

// parseHostDir extracts (scheduleUUID, sqlVersion) from a host directory
// path. Used so the retention sweeper can look up the right retention
// setting for a given partition.
func parseHostDir(dir, root string) (uuid.UUID, int32, error) {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return uuid.UUID{}, 0, err
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 3 {
		return uuid.UUID{}, 0, errors.New("unexpected path depth")
	}
	q := strings.TrimPrefix(parts[0], "query=")
	v := strings.TrimPrefix(parts[1], "v=")
	scheduleUUID, err := uuid.Parse(q)
	if err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("parse schedule uuid: %w", err)
	}
	ver, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("parse sql version: %w", err)
	}
	return scheduleUUID, int32(ver), nil
}

// escapeSQLLiteral escapes ' for use inside a SQL string literal. DuckDB
// follows standard SQL escaping (doubled single quotes). File paths are
// supplied by the server (not user input) so this is defense-in-depth.
func escapeSQLLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
