package monitor

import (
	"time"

	"github.com/venkytv/nats-heartbeat/pkg/heartbeat"
)

type state struct {
	subject     string
	description string
	host        string
	lastSeen    time.Time
	interval    time.Duration
	grace       *time.Duration
	alertActive bool
	missCount   int
	lastAlert   time.Time
}

func newState(msg heartbeat.Message) state {
	return state{
		subject:     msg.Subject,
		description: descriptionOrSubject(msg),
		host:        msg.Host,
		lastSeen:    msg.GeneratedAt,
		interval:    msg.Interval,
		grace:       msg.GracePeriod,
	}
}

func (s state) allowedWindow() time.Duration {
	if s.grace != nil && *s.grace > 0 {
		return *s.grace
	}
	return s.interval
}

func descriptionOrSubject(msg heartbeat.Message) string {
	if msg.Description != "" {
		return msg.Description
	}
	return msg.Subject
}
