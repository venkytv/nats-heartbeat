package monitor

import (
	"testing"
	"time"

	"github.com/venkytv/nats-heartbeat/pkg/heartbeat"
)

func TestAllowedWindowUsesGraceWhenSet(t *testing.T) {
	grace := 5 * time.Second
	st := newState(heartbeat.Message{
		Subject:     "svc",
		GeneratedAt: time.Now(),
		Interval:    3 * time.Second,
		GracePeriod: &grace,
	})
	if got := st.allowedWindow(); got != grace {
		t.Fatalf("expected grace %s, got %s", grace, got)
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

func TestNewStateCapturesHost(t *testing.T) {
	st := newState(heartbeat.Message{
		Subject:     "svc",
		GeneratedAt: time.Now(),
		Interval:    time.Second,
		Host:        "host-1",
	})
	if st.host != "host-1" {
		t.Fatalf("expected host host-1, got %s", st.host)
	}
}
