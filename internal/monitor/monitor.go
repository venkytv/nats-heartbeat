package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/venkytv/nats-heartbeat/internal/notifier"
	"github.com/venkytv/nats-heartbeat/pkg/heartbeat"
)

type Config struct {
	Prefix      string
	PrimeStream string
	PollEvery   time.Duration
	Debug       bool
	Logger      *slog.Logger
	RepeatEvery time.Duration
}

type Monitor struct {
	cfg      Config
	nc       *nats.Conn
	notifier notifier.Notifier
	logger   *slog.Logger

	mu    sync.Mutex
	state map[string]*state
}

func New(nc *nats.Conn, n notifier.Notifier, cfg Config) *Monitor {
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = time.Second
	}
	if cfg.RepeatEvery <= 0 {
		cfg.RepeatEvery = 12 * time.Hour
	}
	cfg.Prefix = strings.TrimSuffix(cfg.Prefix, ".")
	logger := cfg.Logger
	if logger == nil {
		level := slog.LevelInfo
		if cfg.Debug {
			level = slog.LevelDebug
		}
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		}))
	}
	return &Monitor{
		cfg:      cfg,
		nc:       nc,
		notifier: n,
		logger:   logger,
		state:    make(map[string]*state),
	}
}

func (m *Monitor) Start(ctx context.Context) error {
	if m.nc == nil {
		return errors.New("nats connection is required")
	}
	if m.notifier == nil {
		m.notifier = notifier.Nop{}
	}
	if m.cfg.PrimeStream != "" {
		if err := m.primeCache(ctx); err != nil {
			m.logger.Warn("prime cache failed", "err", err)
		}
	}

	subject := m.subscribeSubject()
	sub, err := m.nc.Subscribe(subject, func(msg *nats.Msg) {
		m.handleMessage(ctx, msg)
	})
	if err != nil {
		return err
	}
	m.logger.Info("monitor subscribed", "subject", subject, "prime_stream", m.cfg.PrimeStream)
	defer sub.Unsubscribe()

	ticker := time.NewTicker(m.cfg.PollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("monitor stopping")
			return nil
		case <-ticker.C:
			m.scan(ctx)
		}
	}
}

func (m *Monitor) handleMessage(ctx context.Context, msg *nats.Msg) {
	hb, err := heartbeat.Unmarshal(msg.Data)
	if err != nil {
		m.logger.Error("failed to decode heartbeat", "subject", msg.Subject, "err", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.state[hb.Subject]
	if !ok {
		newState := newState(hb)
		m.state[hb.Subject] = &newState
		m.logger.Debug("new heartbeat subject added", "subject", hb.Subject, "interval", hb.Interval, "skippable", hb.Skippable, "grace", hb.GracePeriod)
		return
	}

	s.lastSeen = hb.GeneratedAt
	s.interval = hb.Interval
	s.skippable = hb.Skippable
	s.grace = hb.GracePeriod
	s.description = descriptionOrSubject(hb)
	m.logger.Debug("heartbeat updated", "subject", hb.Subject, "interval", hb.Interval, "skippable", hb.Skippable, "grace", hb.GracePeriod)

	if s.alertActive {
		s.alertActive = false
		s.missCount = 0
		go m.notifier.Resolved(ctx, notifier.Event{
			Subject:     s.subject,
			Description: s.description,
			LastSeen:    s.lastSeen,
			Interval:    s.interval,
		})
		m.logger.Debug("resolved state on heartbeat", "subject", s.subject, "last_seen", s.lastSeen)
	}
}

func (m *Monitor) scan(ctx context.Context) {
	now := time.Now()
	var toAlert []notifier.Event
	var toResolve []notifier.Event

	m.mu.Lock()
	for _, s := range m.state {
		elapsed := now.Sub(s.lastSeen)
		allowed := s.allowedWindow()

		if elapsed <= allowed {
			if s.alertActive {
				toResolve = append(toResolve, notifier.Event{
					Subject:     s.subject,
					Description: s.description,
					LastSeen:    s.lastSeen,
					Interval:    s.interval,
					MissFor:     elapsed,
					MissCount:   s.missCount,
				})
				s.alertActive = false
				s.missCount = 0
				s.lastAlert = time.Time{}
				m.logger.Debug("heartbeat recovered", "subject", s.subject, "elapsed", elapsed, "allowed", allowed)
			}
			continue
		}

		s.missCount = int(elapsed / s.interval)
		if !s.alertActive {
			toAlert = append(toAlert, notifier.Event{
				Subject:     s.subject,
				Description: s.description,
				LastSeen:    s.lastSeen,
				Interval:    s.interval,
				MissFor:     elapsed,
				MissCount:   s.missCount,
			})
			s.alertActive = true
			s.lastAlert = now
			m.logger.Debug("heartbeat missed threshold", "subject", s.subject, "elapsed", elapsed, "allowed", allowed, "miss_count", s.missCount)
		} else if now.Sub(s.lastAlert) >= m.cfg.RepeatEvery {
			toAlert = append(toAlert, notifier.Event{
				Subject:     s.subject,
				Description: s.description,
				LastSeen:    s.lastSeen,
				Interval:    s.interval,
				MissFor:     elapsed,
				MissCount:   s.missCount,
			})
			s.lastAlert = now
			m.logger.Debug("heartbeat still missing, repeating alert", "subject", s.subject, "elapsed", elapsed, "allowed", allowed, "miss_count", s.missCount, "repeat_every", m.cfg.RepeatEvery)
		}
	}
	m.mu.Unlock()

	for _, evt := range toAlert {
		if err := m.notifier.Alert(ctx, evt); err != nil {
			m.logger.Error("alert notify failed", "subject", evt.Subject, "err", err)
		}
	}
	for _, evt := range toResolve {
		if err := m.notifier.Resolved(ctx, evt); err != nil {
			m.logger.Error("resolved notify failed", "subject", evt.Subject, "err", err)
		}
	}
}

func (m *Monitor) primeCache(ctx context.Context) error {
	js, err := m.nc.JetStream()
	if err != nil {
		return err
	}

	subject := m.subscribeSubject()
	m.logger.Info("priming cache from stream", "stream", m.cfg.PrimeStream, "subject", subject)
	sub, err := js.SubscribeSync(subject,
		nats.BindStream(m.cfg.PrimeStream),
		nats.ManualAck(),
		nats.DeliverLastPerSubject(),
		nats.MaxDeliver(1),
	)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for {
		msg, err := sub.NextMsgWithContext(timeoutCtx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		m.handleMessage(ctx, msg)
		_ = msg.Ack()
	}
}

func (m *Monitor) subscribeSubject() string {
	if m.cfg.Prefix == "" {
		return ">"
	}
	return fmt.Sprintf("%s.>", m.cfg.Prefix)
}
