package notifier

import (
	"context"
	"time"
)

// Event captures alert or resolution details.
type Event struct {
	Subject     string
	Description string
	LastSeen    time.Time
	Interval    time.Duration
	MissCount   int
	MissFor     time.Duration
}

// Notifier sends alerts and resolutions to downstream channels.
type Notifier interface {
	Alert(ctx context.Context, evt Event) error
	Resolved(ctx context.Context, evt Event) error
}

// Nop is a no-op notifier useful in tests.
type Nop struct{}

func (Nop) Alert(_ context.Context, _ Event) error    { return nil }
func (Nop) Resolved(_ context.Context, _ Event) error { return nil }
