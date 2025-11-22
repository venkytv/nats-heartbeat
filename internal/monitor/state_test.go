package monitor

import (
	"testing"
	"time"

	"github.com/venkytv/nats-heartbeat/pkg/heartbeat"
)

func TestAllowedWindowPrefersSmallest(t *testing.T) {
	skips := 2
	grace := 5 * time.Second
	st := newState(heartbeat.Message{
		Subject:     "svc",
		GeneratedAt: time.Now(),
		Interval:    3 * time.Second,
		Skippable:   &skips,
		GracePeriod: &grace,
	})
	// Skips would allow 9s, grace is 5s; expect 5s window.
	if got := st.allowedWindow(); got != grace {
		t.Fatalf("expected %s, got %s", grace, got)
	}
}

func TestAllowedWindowDefaultsToInterval(t *testing.T) {
	st := newState(heartbeat.Message{
		Subject:     "svc",
		GeneratedAt: time.Now(),
		Interval:    time.Second,
	})
	if got := st.allowedWindow(); got != time.Second {
		t.Fatalf("expected interval %s, got %s", time.Second, got)
	}
}
