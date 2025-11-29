package monitor

import (
	"testing"
	"time"
)

func TestSnapshotReportsMissingAndHealthy(t *testing.T) {
	m := New(nil, nil, Config{})
	now := time.Now()

	grace := 2 * time.Second

	m.mu.Lock()
	m.state["svc-missing"] = &state{
		subject:     "svc-missing",
		description: "svc-missing",
		host:        "host-a",
		lastSeen:    now.Add(-3 * time.Second),
		interval:    time.Second,
	}
	m.state["svc-healthy"] = &state{
		subject:     "svc-healthy",
		description: "svc-healthy",
		host:        "host-b",
		lastSeen:    now.Add(-500 * time.Millisecond),
		interval:    time.Second,
		grace:       &grace,
	}
	m.mu.Unlock()

	snapshot := m.snapshot(now)
	if len(snapshot) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(snapshot))
	}

	var missing, healthy subjectState
	if snapshot[0].Subject == "svc-healthy" {
		healthy = snapshot[0]
		missing = snapshot[1]
	} else {
		missing = snapshot[0]
		healthy = snapshot[1]
	}

	if !missing.Missing {
		t.Fatalf("expected svc-missing to be missing")
	}
	if missing.MissFor == "" || missing.MissFor != "3s" {
		t.Fatalf("expected svc-missing miss_for 3s, got %q", missing.MissFor)
	}
	if missing.MissCount != 3 {
		t.Fatalf("expected miss count 3, got %d", missing.MissCount)
	}
	if missing.Host != "host-a" {
		t.Fatalf("expected host host-a, got %s", missing.Host)
	}

	if healthy.Missing {
		t.Fatalf("expected svc-healthy to be healthy")
	}
	if healthy.MissFor != "" {
		t.Fatalf("expected empty miss_for for healthy subject, got %q", healthy.MissFor)
	}
	if healthy.Grace == nil || *healthy.Grace != grace.String() {
		t.Fatalf("expected grace %s, got %v", grace, healthy.Grace)
	}
	if healthy.AllowedWindow != grace.String() {
		t.Fatalf("expected allowed window %s, got %s", grace, healthy.AllowedWindow)
	}
	if healthy.Host != "host-b" {
		t.Fatalf("expected host host-b, got %s", healthy.Host)
	}
}
