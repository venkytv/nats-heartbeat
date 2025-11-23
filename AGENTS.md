# Repository Guidelines

## Project Structure & Module Organization
- Module at repo root; binaries in `cmd/agent/` and `cmd/monitor/`. Shared code in `internal/`; reusable APIs for other Go tools in `pkg/`.
- Co-locate tests with source files as `_test.go`; keep integration fixtures in `testdata/`.
- Sample env/configs go in `configs/` (e.g., `configs/sample.env`); `.env` is ignored.

## Architecture Overview
- Library publishes heartbeats (identity, generated time, expected interval) on NATS subjects under a prefix; optional grace duration and description (fallback: subject).
- Monitor subscribes to that prefix, caches last-seen per subject, triggers when the grace (or interval when grace is unset) is breached, and emits resolved notices when beats resume.
- Optional: prime the cache at startup from a NATS stream of latest messages. Notifier interface is pluggable; default channel is Pushover.

## Build, Test, and Development Commands
- `go fmt ./...` and `goimports -w ./...` keep formatting consistent.
- `go vet ./...` surfaces common correctness issues.
- `go test ./...` runs unit/integration suites; add `-cover` for a quick coverage read.
- `go run ./cmd/agent` publishes beats; `go run ./cmd/monitor` consumes, evaluates thresholds, and sends notifications.
- `go mod tidy` trims dependencies after adding imports.

## Coding Style & Naming Conventions
- Standard Go style; exported identifiers need GoDoc on behavior.
- Domain-focused names: `HeartbeatPublisher`, `MissedBeatChecker`, `Notifier`, `NATSClient`.
- Pass contexts for cancellation/deadlines on external calls; wrap errors with `%w`.
- Keep packages cohesive; avoid cycles and minimize public surface via `internal/`.

## Testing Guidelines
- Table-driven tests named `TestFunctionName_Scenario`.
- Gate NATS-dependent tests with build tags (e.g., `//go:build integration`); skip when env vars are missing.
- Cover late beats, consecutive misses, jitter, reconnect paths; favor lightweight fakes over global state.

## Commit & Pull Request Guidelines
- Imperative commit subjects stating the final outcome (e.g., `Add missed-beat alert threshold`); keep scope tight.
- Bodies explain the “why” and wrap near 72 chars.
- PRs include a concise summary, related issues, and test evidence (`go test ./...` or notes on skipped integration runs).
- Call out config changes (env vars, ports) and update sample configs alongside code.

## Security & Configuration Tips
- Never commit secrets or live NATS credentials; use `.env` locally and rotate tokens regularly.
- Validate inputs from NATS subjects; treat payloads as untrusted and fail fast with clear logs.
- Keep notifier interfaces small so additional channels can be added without touching core monitor logic; default remains Pushover.
