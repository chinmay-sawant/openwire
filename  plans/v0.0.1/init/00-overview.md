# OpenWire — Portmaster-Inspired Go TUI

> **Parent:** none — initial product and architecture plan
> **Status:** proposed; no implementation started
> **Estimated effort:** 5–8 implementation slices for the first usable TUI, followed by optional live-source and web-reporting work

---

## Plan Index

This directory is the split form of the original `openwire-portmaster-inspired-tui.md` plan. This file is the canonical overview and navigation entrypoint; the phase files contain the live checklist rows.

- [Phase 0 — Product and Runtime Contract](01-phase-0-product-runtime-contract.md)
- [Phase 1 — Go Module and Command Skeleton](02-phase-1-go-module-command-skeleton.md)
- [Phase 2 — Domain and Application Architecture](03-phase-2-domain-application-architecture.md)
- [Phase 3 — Portmaster-Compatible Read-Only Source Adapter](04-phase-3-portmaster-read-only-adapter.md)
- [Phase 4 — TUI Vertical Slice](05-phase-4-tui-vertical-slice.md)
- [Phase 5 — Safe Controls and Operational Views](06-phase-5-safe-controls-operational-views.md)
- [Phase 6 — Release and Validation Closure](07-phase-6-release-validation.md)
- [Future Phase 7 — Gin API and React Reporting](08-phase-7-gin-react-reporting.md)
- [Future Phase 8 — OpenWire Collector and Privileged Boundary](09-phase-8-openwire-collector.md)
- [Dependencies, Non-Goals, and Open Decisions](10-dependencies-non-goals-decisions.md)

## Overview

OpenWire will be a Go application inspired by Portmaster’s network-visibility and policy-management experience. The first deliverable is a terminal user interface (TUI). The initial TUI is a user-context client and visualization layer; it does not own packet interception, firewall enforcement, DNS interception, kernel drivers, system-service installation, or privileged platform integration.

The reference scan covered the separate `portmaster/` repository. Portmaster’s README describes a privileged core service with a separate user-facing UI, and its Go code exposes HTTP and WebSocket seams under `/api/v1/`. Its `service.Instance` composition root wires a very large set of concrete modules, while the existing desktop client is Angular/Tauri rather than Go. OpenWire should preserve the useful separation without importing Portmaster’s internal module graph.

This plan follows `skills/SKILLS.md`: one canonical plan, dependency-ordered phases, atomic checklist rows, explicit proof gates, and no status closure without current evidence.

## Executive Summary

### Recommended runtime model

The first application runs as a foreground process owned by the user:

```text
# deterministic local development and demo mode
openwire tui --demo

# live read-only mode against a compatible local service
openwire tui --endpoint http://127.0.0.1:817
```

Runtime decisions for the first release:

- `openwire tui` is the only product command in the first milestone.
- `--demo` uses deterministic in-memory data and requires no network, credentials, or elevated privileges.
- Live mode uses an explicit endpoint and credential source; it never silently falls back to demo mode.
- The TUI is read-only in the first vertical slice. Any mutation must be capability-gated and explicitly confirmed in a later phase.
- The TUI does not start or install a daemon. A future `openwired` process may own privileged collection, while `openwire tui` remains an unprivileged client.
- A future `openwire serve` command may expose the same application service through Gin for a React reporting client. That transport is not part of the first TUI implementation.
- The initial live adapter may consume a Portmaster-compatible HTTP/WebSocket API, but OpenWire’s long-term application contract must not expose Portmaster’s raw records or database protocol.

### Target architecture

```text
cmd/openwire
    |
    +--> application facade / use cases
              |
              +--> domain read models and typed ports
              |        ^
              |        |
              +--> demo source
              +--> Portmaster-compatible source
              +--> future OpenWire collector source
                       |
                    system/network adapters (future, privileged boundary)

TUI adapter ---------------------> application facade
Future Gin HTTP/WebSocket --------> application facade
Future React --------------------> versioned HTTP/JSON + event contracts
```

The TUI must not import Bubble Tea/tcell types into domain or application packages. Future Gin handlers must not appear below the HTTP adapter. Raw Portmaster `record.Record`, `api.Request`, module event managers, and database-key strings must remain inside compatibility adapters.

## Reference Scan Baseline

These are current observations from `portmaster/`, not claims about OpenWire implementation:

- Product behavior includes network activity monitoring, blocking/allowing, per-app settings, secure DNS, history, bandwidth visibility, and SPN; see `portmaster/README.md:24-85`.
- The core is a system service and the UI runs separately; see `portmaster/README.md:41-53`.
- The primary daemon uses Cobra and delegates service startup from `portmaster/cmds/portmaster-core/main.go:21-84`.
- `service.Instance` constructs and owns many modules, with ordered start/stop groups in `portmaster/service/instance.go` and `portmaster/service/mgr/group.go`.
- The API exposes HTTP/WebSocket functionality and database operations under `/api/v1/`; see `portmaster/base/api/router.go`, `portmaster/base/api/database.go`, and `portmaster/base/api/client/`.
- Useful read models include system status and network connection records; see `portmaster/service/status/` and `portmaster/service/network/`.
- The reference uses loopback HTTP by default but supports configurable addresses and multiple authentication methods; see `portmaster/base/api/config.go` and `portmaster/base/api/authentication.go`.
- Linux and Windows interception, capabilities, services, and drivers are platform-specific and privileged; see `portmaster/service/firewall/interception/` and `portmaster/packaging/`.
- The reference has no Go TUI, no typed TUI-facing client facade, and limited end-to-end coverage for authenticated HTTP/WebSocket reconnect behavior. OpenWire must introduce those seams deliberately.

## Navigation and Status Rules

- `[ ]` means not started or not proven.
- `[x]` means implemented and validated with current evidence.
- `[~]` means intentionally deferred or partial, with reason, owner boundary, and next gate.
- Phase files are not complete until their checklist rows and required proof are synchronized here and in the relevant phase file.
- Documentation-only reorganization does not close implementation rows and does not require lint or test checks.
