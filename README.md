# nats-heartbeat

Lightweight NATS-driven liveness agent and monitor. Agents publish heartbeats (with expected interval and optional thresholds); the monitor consumes them, detects consecutive misses or elapsed grace windows, and sends alerts (default: Pushover) plus a resolved notice when beats resume. A reusable library lets other Go binaries emit heartbeats directly.

## Prereqs
- Go 1.21+
- NATS server reachable by the agent/monitor
- Pushover credentials for notifications (or swap in another notifier)

## Agent (CLI)
Publishes periodic heartbeats on a full subject you choose (e.g., `heartbeat.my-service`).

```sh
go run ./cmd/agent \
  -nats-url nats://localhost:4222 \
  -subject heartbeat.service.api \
  -interval 15s \
  -grace 0 \
  -description "API service"
```

Flags (env mirrors in parentheses):
- `-nats-url` (`NATS_URL`): NATS server URL.
- `-subject` (`SUBJECT`): required full subject per service (recommend a `heartbeat.*` namespace).
- `-interval` (`INTERVAL`): heartbeat period (e.g., `15s`).
- `-grace` (`GRACE`): duration allowed with no beats; omit/0 to fall back to interval.
- `-description` (`DESCRIPTION`): human-friendly label (falls back to subject).

Each heartbeat includes the originating host (defaults to the local hostname), interval, and optional grace/description metadata.

## Monitor (CLI)
Watches a subject prefix, evaluates miss thresholds, and notifies when breached or resolved.

```sh
go run ./cmd/monitor \
  -nats-url nats://localhost:4222 \
  -subject-prefix heartbeat. \
  -prime-stream HEARTBEATS \
  -poll 1s \
  -repeat-every 12h \
  -pushover-user "$PUSHOVER_USER" \
  -pushover-token "$PUSHOVER_TOKEN"
```

Flags (env mirrors in parentheses):
- `-nats-url` (`NATS_URL`): NATS server URL.
- `-subject-prefix` (`SUBJECT_PREFIX`, default `heartbeat.`): prefix to subscribe to.
- `-prime-stream` (`PRIME_STREAM`): optional JetStream stream name to seed last-seen messages once on startup (uses deliver-last-per-subject).
- `-poll` (`POLL_INTERVAL`): scan cadence for missed beats.
- `-repeat-every` (`REPEAT_EVERY`, default `12h`): how often to repeat alerts while a heartbeat remains missing.
- `-pushover-user` (`PUSHOVER_USER`), `-pushover-token` (`PUSHOVER_TOKEN`): Pushover credentials.

Behavior:
- Uses grace duration as the miss window (falls back to interval when grace is unset/0).
- Caches last-seen per subject in memory.
- Sends a resolved notification when heartbeats resume.
- Repeats alerts at the configured interval while a heartbeat is still missing.
- Notifier interface is pluggable; Pushover is the default implementation.

### Clearing obsolete heartbeats when using cache priming
If a service is retired and you use JetStream priming, remove its last-seen message from the stream so it stops alerting. With the NATS CLI:

```sh
nats stream purge HEARTBEATS --subject heartbeat.retired-service --force
```

Replace `HEARTBEATS` with your stream name and `heartbeat.retired-service` with the subject to clear.

## Status (CLI)
Query the monitor's status endpoint (default `http://127.0.0.1:8080/`) and highlight any firing alerts:

```sh
go run ./cmd/status -url http://127.0.0.1:8080/
```

Flags (env mirrors in parentheses):
- `-url` (`STATUS_URL`): status endpoint URL.
- `-timeout` (`STATUS_TIMEOUT`, default `3s`): HTTP request timeout.

Example output:

```
Observed at: 2024-06-01T12:00:00Z

STATUS  SUBJECT                DESCRIPTION        HOST      LAST SEEN                 DETAILS
ALERT!  heartbeat.api          API service        host-a    2024-06-01T11:59:30Z      missed 30s (2 beats)
OK      heartbeat.worker.queue Worker processor   host-b    2024-06-01T11:59:55Z      interval 10s, window 10s

1 alert(s) firing across 2 subject(s)
```

## Library Usage (publish heartbeats)
Embed heartbeat publishing in your own Go binaries:

```go
package main

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/venkytv/nats-heartbeat/pkg/heartbeat"
)

func main() {
	nc, _ := nats.Connect(nats.DefaultURL)
	pub := heartbeat.NewPublisher(nc, "heartbeat.")

	msg := heartbeat.Message{
		Subject:     "heartbeat.service.api",
		GeneratedAt: time.Now().UTC(),
		Interval:    15 * time.Second,
		Host:        "api-host-1", // optional; defaults to local hostname
		Description: "API service",
		// Optional threshold:
		// GracePeriod: func(d time.Duration) *time.Duration { return &d }(45 * time.Second),
	}
	_ = pub.Publish(context.Background(), msg)
}
```
