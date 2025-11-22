# Changelog

All notable changes to this project will be documented in this file.

## Unreleased
- Add changelog and documentation updates.

## v0.1.0
- Introduce Go module `github.com/venkytv/nats-heartbeat`.
- Add heartbeat library with message validation, JSON marshal/unmarshal, and NATS publisher.
- Add monitor with in-memory cache, JetStream priming option, threshold evaluation (skips vs grace), resolved notifications, and pluggable notifier interface (default Pushover).
- Add agent CLI to publish heartbeats on configured subjects with optional skip/grace/description fields.
- Add monitor CLI with subject-prefix subscription, cache priming, and structured logging with debug mode.
