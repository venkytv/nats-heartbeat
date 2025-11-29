package heartbeat

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// Publisher sends heartbeat messages to NATS with a subject prefix.
type Publisher struct {
	nc     *nats.Conn
	prefix string
}

var hostname = os.Hostname

func NewPublisher(nc *nats.Conn, prefix string) *Publisher {
	return &Publisher{
		nc:     nc,
		prefix: strings.TrimSuffix(prefix, "."),
	}
}

// Publish sends a heartbeat to NATS using the configured prefix.
func (p *Publisher) Publish(ctx context.Context, msg Message) error {
	if msg.GeneratedAt.IsZero() {
		msg.GeneratedAt = time.Now().UTC()
	}
	msg = applyHostDefault(msg)
	if err := msg.Validate(); err != nil {
		return err
	}
	payload, err := msg.Marshal()
	if err != nil {
		return err
	}
	subject := p.fullSubject(msg.Subject)
	return p.nc.PublishMsg(&nats.Msg{
		Subject: subject,
		Data:    payload,
		Header:  cloneHeaders(ctx),
	})
}

func (p *Publisher) fullSubject(s string) string {
	if p.prefix == "" {
		return s
	}
	return fmt.Sprintf("%s.%s", p.prefix, s)
}

func applyHostDefault(msg Message) Message {
	if msg.Host != "" {
		return msg
	}
	if host, err := hostname(); err == nil && host != "" {
		msg.Host = host
	}
	return msg
}

// cloneHeaders extracts trace-like metadata from context if available.
func cloneHeaders(ctx context.Context) nats.Header {
	headers := nats.Header{}
	if ctx == nil {
		return headers
	}
	if deadline, ok := ctx.Deadline(); ok {
		headers.Set("Deadline", deadline.UTC().Format(time.RFC3339Nano))
	}
	return headers
}
