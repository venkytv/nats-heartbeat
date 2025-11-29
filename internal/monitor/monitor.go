package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"
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
	StatusAddr  string
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

	var statusErrCh chan error
	if m.cfg.StatusAddr != "" {
		statusErrCh = make(chan error, 1)
		go m.serveStatus(ctx, statusErrCh)
	}

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("monitor stopping")
			return nil
		case err, ok := <-statusErrCh:
			if ok && err != nil {
				return fmt.Errorf("status server: %w", err)
			}
			if !ok {
				statusErrCh = nil
			}
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
		m.logger.Debug("new heartbeat subject added", "subject", hb.Subject, "interval", hb.Interval, "grace", hb.GracePeriod)
		return
	}

	s.lastSeen = hb.GeneratedAt
	s.interval = hb.Interval
	s.grace = hb.GracePeriod
	s.host = hb.Host
	s.description = descriptionOrSubject(hb)
	m.logger.Debug("heartbeat updated", "subject", hb.Subject, "interval", hb.Interval, "grace", hb.GracePeriod)

	if s.alertActive {
		s.alertActive = false
		s.missCount = 0
		go m.notifier.Resolved(ctx, notifier.Event{
			Subject:     s.subject,
			Description: s.description,
			Host:        s.host,
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
					Host:        s.host,
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
				Host:        s.host,
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
				Host:        s.host,
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

type statusResponse struct {
	ObservedAt time.Time      `json:"observed_at"`
	Subjects   []subjectState `json:"subjects"`
}

type subjectState struct {
	Subject       string    `json:"subject"`
	Description   string    `json:"description"`
	Host          string    `json:"host,omitempty"`
	LastSeen      time.Time `json:"last_seen"`
	Interval      string    `json:"interval"`
	Grace         *string   `json:"grace,omitempty"`
	AllowedWindow string    `json:"allowed_window"`
	Missing       bool      `json:"missing"`
	MissFor       string    `json:"miss_for,omitempty"`
	MissCount     int       `json:"miss_count,omitempty"`
	AlertActive   bool      `json:"alert_active"`
}

func (m *Monitor) serveStatus(ctx context.Context, errCh chan<- error) {
	server := &http.Server{
		Addr:    m.cfg.StatusAddr,
		Handler: m.statusHandler(),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.logger.Warn("status server shutdown failed", "err", err)
		}
	}()

	m.logger.Info("status server starting", "addr", m.cfg.StatusAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errCh <- err
	}
	close(errCh)
}

func (m *Monitor) statusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedAt := time.Now()
		resp := statusResponse{
			ObservedAt: observedAt,
			Subjects:   m.snapshot(observedAt),
		}

		w.Header().Set("Content-Type", "application/json")

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(resp); err != nil {
			m.logger.Warn("status response encode failed", "err", err)
			http.Error(w, "encode failed", http.StatusInternalServerError)
		}
	})
}

func (m *Monitor) snapshot(now time.Time) []subjectState {
	m.mu.Lock()
	defer m.mu.Unlock()

	subjects := make([]subjectState, 0, len(m.state))
	for _, s := range m.state {
		allowed := s.allowedWindow()
		elapsed := now.Sub(s.lastSeen)
		missing := elapsed > allowed

		var missFor string
		var missCount int
		if missing {
			missFor = elapsed.String()
			if s.interval > 0 {
				missCount = int(elapsed / s.interval)
			}
		}

		subject := subjectState{
			Subject:       s.subject,
			Description:   s.description,
			Host:          s.host,
			LastSeen:      s.lastSeen,
			Interval:      s.interval.String(),
			AllowedWindow: allowed.String(),
			Missing:       missing,
			MissFor:       missFor,
			MissCount:     missCount,
			AlertActive:   s.alertActive,
		}
		if s.grace != nil && *s.grace > 0 {
			grace := (*s.grace).String()
			subject.Grace = &grace
		}

		subjects = append(subjects, subject)
	}

	sort.Slice(subjects, func(i, j int) bool {
		return subjects[i].Subject < subjects[j].Subject
	})

	return subjects
}
