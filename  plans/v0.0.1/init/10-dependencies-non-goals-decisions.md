# OpenWire — Dependencies, Non-Goals, and Open Decisions

> **Parent:** [00-overview.md](00-overview.md)
> **Status:** proposed; no implementation started
> **Estimated effort:** maintained alongside phase execution

---

## Dependencies and Non-Goals

### Dependency order

```text
Runtime contract
      |
Go skeleton ---> domain/application ports ---> demo source ---> TUI vertical slice
                                      |
                                      +--> Portmaster read-only adapter
                                      |
                                      +--> safe controls/history/reports
                                      |
                                      +--> Gin contracts + React
                                      |
                                      +--> optional privileged collector
```

### Explicit first-release non-goals

- Packet capture/interception or firewall enforcement.
- DNS interception or resolver ownership.
- eBPF, nfqueue, WFP, kernel modules, driver installation, or process ownership detection.
- SPN tunnel construction, node discovery, accounts, billing, or subscriptions.
- System-service installation, tray integration, desktop notifications, installers, and auto-updates.
- Full parity with Portmaster’s Angular/Tauri screens or expert settings.
- Direct database access from the TUI.
- Automatic fallback from a failed live endpoint to demo data.
- Treating Portmaster’s raw API/database protocol as OpenWire’s long-term public API.

## Open Decisions Before Implementation

These choices should be resolved in Phase 0 rather than assumed in implementation code:

1. Final module path and product command name (`openwire` assumed here).
2. TUI framework and minimum terminal size.
3. Linux-only first or Linux plus Windows/macOS for the client process.
4. Whether the first live source is Portmaster-compatible API, a new OpenWire collector, or demo/replay only.
5. Credential mechanism for live mode and whether local process authentication is available.
6. Whether any mutations belong in the first release or all controls remain read-only.
7. Whether local TUI preferences need persistence in the first release.
8. Whether reports are terminal-only initially or include JSON/YAML export from Phase 5.
9. The exact future Gin/React authentication and remote-binding policy.
