# OpenWire v0.0.2 — SQLite Persistence

> **Parent:** [v0.0.1 init](../../v0.0.1/init/00-overview.md)  
> **Status:** in progress  
> **Goal:** optional on-disk history for bandwidth samples and app totals without changing the live TUI hot path

## Approach

- Keep `internal/store/memory` as the live read/write store for the UI.
- Add `internal/store/sqlite` **recorder** that:
  - opens/creates a SQLite file (pure Go: `modernc.org/sqlite`)
  - flushes samples + app usage snapshots on an interval
  - loads recent samples into memory on startup (graph continuity)
- CLI: `--db PATH` (default: `$XDG_STATE_HOME/openwire/openwire.db`), `--no-db` to disable.

## Non-goals (this slice)

- Full packet PCAP on disk
- Multi-user / remote DB
- Replacing in-memory ranking with SQL queries in the TUI

## Checklist

- [x] Schema: samples, app_snapshots, meta
- [x] Recorder flush + load
- [x] Memory helpers for load/export (`LoadSamples`, `SamplesSince`)
- [x] CLI flags + default path (`--db`, `--no-db`)
- [x] Tests (temp DB)
- [x] README / runtime docs
