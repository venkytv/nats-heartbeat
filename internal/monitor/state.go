package monitor

import (
	"time"

	"github.com/venkytv/nats-heartbeat/pkg/heartbeat"
)

type state struct {
	subject     string
	description string
	lastSeen    time.Time
	interval    time.Duration
	skippable   *int
	grace       *time.Duration
	alertActive bool
	missCount   int
	lastAlert   time.Time
}

func newState(msg heartbeat.Message) state {
	return state{
		subject:     msg.Subject,
		description: descriptionOrSubject(msg),
		lastSeen:    msg.GeneratedAt,
		interval:    msg.Interval,
		skippable:   msg.Skippable,
		grace:       msg.GracePeriod,
	}
}

func (s state) allowedWindow() time.Duration {
	allowed := s.interval
	if s.skippable != nil {
		allowed = s.interval * time.Duration(*s.skippable+1)
	}
	if s.grace != nil && *s.grace > 0 {
		if allowed <= 0 || *s.grace < allowed {
			allowed = *s.grace
		}
	}
	return allowed
}

func descriptionOrSubject(msg heartbeat.Message) string {
	if msg.Description != "" {
		return msg.Description
	}
	return msg.Subject
}
