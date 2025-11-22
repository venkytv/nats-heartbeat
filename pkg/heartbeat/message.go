package heartbeat

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Message describes a heartbeat payload exchanged over NATS.
type Message struct {
	Subject     string         `json:"subject"`
	GeneratedAt time.Time      `json:"generated_at"`
	Interval    time.Duration  `json:"interval"`               // expected heartbeat period
	Skippable   *int           `json:"skippable,omitempty"`    // number of beats allowed to miss
	GracePeriod *time.Duration `json:"grace_period,omitempty"` // max time to miss beats
	Description string         `json:"description,omitempty"`
}

// Marshal renders the message as JSON for transport.
func (m Message) Marshal() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// Unmarshal decodes a heartbeat message from JSON.
func Unmarshal(data []byte) (Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return Message{}, err
	}
	return msg, msg.Validate()
}

// Validate ensures required fields are present and well-formed.
func (m Message) Validate() error {
	if m.Subject == "" {
		return errors.New("subject is required")
	}
	if m.GeneratedAt.IsZero() {
		return errors.New("generated_at is required")
	}
	if m.Interval <= 0 {
		return fmt.Errorf("interval must be >0, got %s", m.Interval)
	}
	if m.Skippable != nil && *m.Skippable < 0 {
		return errors.New("skippable cannot be negative")
	}
	if m.GracePeriod != nil && *m.GracePeriod < 0 {
		return errors.New("grace period cannot be negative")
	}
	return nil
}
