# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

## v0.2.0
- BREAKING: remove skippable-count threshold; grace duration now defines the miss window (falls back to interval when unset).
- Update agent/monitor docs to reflect grace-only behavior.

## v0.1.0
- Introduce Go module `github.com/venkytv/nats-heartbeat`.
- Add heartbeat library with message validation, JSON marshal/unmarshal, and NATS publisher.
- Add monitor with in-memory cache, JetStream priming option, threshold evaluation (skips vs grace), resolved notifications, and pluggable notifier interface (default Pushover).
- Add agent CLI to publish heartbeats on configured subjects with optional skip/grace/description fields.
- Add monitor CLI with subject-prefix subscription, cache priming, and structured logging with debug mode.
