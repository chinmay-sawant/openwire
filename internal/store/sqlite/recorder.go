// Package sqlite persists bandwidth samples and app snapshots for OpenWire history.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
	"github.com/chinmay-sawant/openwire/internal/store/memory"
	_ "modernc.org/sqlite"
)

// Recorder writes live store snapshots to SQLite and can reload recent samples.
type Recorder struct {
	db       *sql.DB
	path     string
	interval time.Duration
	// retain how many sample rows to keep
	maxSamples int
	lastFlush  time.Time
}

// Open creates or opens a SQLite database at path and migrates schema.
func Open(path string) (*Recorder, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("sqlite mkdir: %w", err)
	}
	// modernc driver name is "sqlite"
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Single writer; keep it simple for a local TUI app.
	db.SetMaxOpenConns(1)
	r := &Recorder{
		db:         db,
		path:       path,
		interval:   2 * time.Second,
		maxSamples: 3600, // ~1h at 1s if fully flushed; pruning applied
	}
	if err := r.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

// Path returns the database file path.
func (r *Recorder) Path() string { return r.path }

// Close flushes nothing extra and closes the DB.
func (r *Recorder) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Recorder) migrate() error {
	_, err := r.db.Exec(`
CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS samples (
  ts_unix_ms INTEGER NOT NULL,
  rx_bps     REAL NOT NULL,
  tx_bps     REAL NOT NULL,
  total_bps  REAL NOT NULL,
  PRIMARY KEY (ts_unix_ms)
);
CREATE TABLE IF NOT EXISTS app_snapshots (
  ts_unix_ms INTEGER NOT NULL,
  app_key    TEXT NOT NULL,
  name       TEXT NOT NULL,
  pid        INTEGER NOT NULL,
  path       TEXT NOT NULL,
  bytes_in   INTEGER NOT NULL,
  bytes_out  INTEGER NOT NULL,
  rate_in    REAL NOT NULL,
  rate_out   REAL NOT NULL,
  flows      INTEGER NOT NULL,
  PRIMARY KEY (ts_unix_ms, app_key)
);
CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(ts_unix_ms);
CREATE INDEX IF NOT EXISTS idx_apps_ts ON app_snapshots(ts_unix_ms);
`)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`INSERT OR REPLACE INTO meta(key, value) VALUES ('schema_version', '1')`)
	return err
}

// LoadRecentSamples returns up to limit samples ordered oldest→newest.
func (r *Recorder) LoadRecentSamples(limit int) ([]domain.BandwidthSample, error) {
	if limit <= 0 {
		limit = 120
	}
	rows, err := r.db.Query(`
SELECT ts_unix_ms, rx_bps, tx_bps, total_bps
FROM samples
ORDER BY ts_unix_ms DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Read newest-first then reverse.
	tmp := make([]domain.BandwidthSample, 0, limit)
	for rows.Next() {
		var ms int64
		var rx, tx, total float64
		if err := rows.Scan(&ms, &rx, &tx, &total); err != nil {
			return nil, err
		}
		tmp = append(tmp, domain.BandwidthSample{
			Time:     time.UnixMilli(ms).UTC(),
			RxBps:    rx,
			TxBps:    tx,
			TotalBps: total,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(tmp)-1; i < j; i, j = i+1, j-1 {
		tmp[i], tmp[j] = tmp[j], tmp[i]
	}
	return tmp, nil
}

// Run periodically flushes new samples and app rankings from mem until ctx ends.
func (r *Recorder) Run(ctx context.Context, mem *memory.Store) {
	if r == nil || mem == nil {
		return
	}
	// Seed lastFlush from newest DB sample so we only append new points.
	if samples, err := r.LoadRecentSamples(1); err == nil && len(samples) > 0 {
		r.lastFlush = samples[0].Time
	}

	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			// final flush
			if err := r.Flush(mem); err != nil {
				slog.Warn("sqlite final flush", "err", err)
			}
			return
		case <-t.C:
			if err := r.Flush(mem); err != nil {
				slog.Warn("sqlite flush", "err", err)
			}
		}
	}
}

// Flush writes new samples and a current app snapshot.
func (r *Recorder) Flush(mem *memory.Store) error {
	if r == nil || mem == nil {
		return nil
	}
	samples := mem.SamplesSince(r.lastFlush)
	apps := mem.ListAppsByBandwidth(50)
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var newest time.Time
	for _, s := range samples {
		ms := s.Time.UTC().UnixMilli()
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO samples(ts_unix_ms, rx_bps, tx_bps, total_bps) VALUES (?,?,?,?)`,
			ms, s.RxBps, s.TxBps, s.TotalBps,
		); err != nil {
			return err
		}
		if s.Time.After(newest) {
			newest = s.Time
		}
	}

	// One app snapshot row set per flush (even if no new samples).
	ts := now.UnixMilli()
	for _, a := range apps {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO app_snapshots(
				ts_unix_ms, app_key, name, pid, path, bytes_in, bytes_out, rate_in, rate_out, flows
			) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			ts, a.Key, a.Name, a.PID, a.Path, a.BytesIn, a.BytesOut, a.RateInBps, a.RateOutBps, a.Flows,
		); err != nil {
			return err
		}
	}

	// Prune old samples.
	if _, err := tx.Exec(`
DELETE FROM samples WHERE ts_unix_ms NOT IN (
  SELECT ts_unix_ms FROM samples ORDER BY ts_unix_ms DESC LIMIT ?
)`, r.maxSamples); err != nil {
		return err
	}
	// Keep app snapshots for last ~500 flush ticks.
	if _, err := tx.Exec(`
DELETE FROM app_snapshots WHERE ts_unix_ms < (
  SELECT COALESCE(MIN(ts_unix_ms), 0) FROM (
    SELECT DISTINCT ts_unix_ms FROM app_snapshots ORDER BY ts_unix_ms DESC LIMIT 500
  )
)`); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if !newest.IsZero() {
		r.lastFlush = newest
	}
	return nil
}
