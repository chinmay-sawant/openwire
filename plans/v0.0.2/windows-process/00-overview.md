# OpenWire v0.0.2 — Windows Process Attribution (WSL2)

> **Parent:** [v0.0.1](../../v0.0.1/init/00-overview.md)  
> **Status:** in progress  
> **Goal:** best-effort per-process bandwidth for **Windows host** apps when OpenWire runs under WSL2

## Approach

From WSL2 (no native Windows binary required for this slice):

1. List Windows TCP/UDP endpoints + owning process via `powershell.exe` (`Get-NetTCPConnection` / `Get-NetUDPEndpoint` + process names).
2. Sample host adapter byte counters (existing).
3. Distribute each interval’s host rx/tx **delta** across Windows processes **by open-connection weight** (same idea as Linux `/proc` stats mode).
4. Show apps as `win/<name>` with Windows PID in the detail pane.

## Limits (honest)

- Not packet-accurate (host NIC counters only).
- Idle processes with no sockets will not appear.
- Requires working `powershell.exe` interop from WSL.
- Not a full Windows ETW/WFP agent.

## Checklist

- [x] List Windows processes with connection weights
- [x] Attribute host adapter deltas per process
- [x] Fallback to aggregate `windows-host` when process list empty
- [x] Wire into WSL sample loop
- [x] Unit tests for attribution math / parsing
- [x] Docs (README + runtime)
