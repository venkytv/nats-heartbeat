package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/venkytv/nats-heartbeat/pkg/heartbeat"
)

func main() {
	var (
		natsURL         = flag.String("nats-url", envDefault("NATS_URL", nats.DefaultURL), "NATS server URL")
		subject         = flag.String("subject", envDefault("SUBJECT", ""), "Full heartbeat subject (required)")
		interval        = flag.Duration("interval", envDuration("INTERVAL", 15*time.Second), "Heartbeat interval")
		grace           = flag.Duration("grace", envDuration("GRACE", 0), "Optional max duration to miss beats before alerting")
		desc            = flag.String("description", envDefault("DESCRIPTION", ""), "Human-friendly description for alerts")
		flushTimeout    = flag.Duration("flush-timeout", envDuration("FLUSH_TIMEOUT", 2*time.Second), "How long to wait for NATS flush after publish")
		exitOnFlushFail = flag.Bool("exit-on-flush-fail", envBool("EXIT_ON_FLUSH_FAIL", false), "Exit when flush fails instead of just logging")
		debug           = flag.Bool("debug", envBool("DEBUG", false), "Enable debug logging")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	if *debug {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}
	slog.SetDefault(logger)

	if *subject == "" {
		log.Fatal("subject is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	nc, err := connectWithRetry(ctx, logger, *natsURL)
	if err != nil {
		logger.Error("connect to nats failed", "err", err)
		return
	}
	defer nc.Drain()

	pub := heartbeat.NewPublisher(nc, "")

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		hb := heartbeat.Message{
			Subject:     *subject,
			GeneratedAt: time.Now().UTC(),
			Interval:    *interval,
			Description: *desc,
		}
		if grace != nil && *grace > 0 {
			hb.GracePeriod = grace
		}

		if err := pub.Publish(ctx, hb); err != nil {
			logger.Error("publish heartbeat failed", "err", err, "subject", hb.Subject)
		} else if err := flushWithTimeout(ctx, nc, *flushTimeout); err != nil {
			logger.Warn("heartbeat flush failed", "err", err, "subject", hb.Subject, "timeout", *flushTimeout)
			if *exitOnFlushFail {
				logger.Error("exiting due to flush failure", "subject", hb.Subject)
				return
			}
		} else {
			logger.Debug("heartbeat published", "subject", hb.Subject, "interval", hb.Interval, "grace", hb.GracePeriod)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || v == "true" || v == "TRUE" || v == "yes" || v == "on"
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func connectWithRetry(ctx context.Context, logger *slog.Logger, url string) (*nats.Conn, error) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		nc, err := nats.Connect(
			url,
			nats.MaxReconnects(-1), // never give up once connected
			nats.ReconnectWait(2*time.Second),
			nats.RetryOnFailedConnect(true), // keep trying initial connects with the same backoff policy
			nats.ErrorHandler(func(_ *nats.Conn, sub *nats.Subscription, err error) {
				if err == nil {
					return
				}
				if sub != nil {
					logger.Warn("nats async error", "err", err, "subject", sub.Subject)
					return
				}
				logger.Warn("nats async error", "err", err)
			}),
			nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
				if err != nil {
					logger.Warn("nats disconnected", "err", err)
					return
				}
				logger.Warn("nats disconnected")
			}),
			nats.ReconnectHandler(func(_ *nats.Conn) {
				logger.Info("nats reconnected")
			}),
			nats.ClosedHandler(func(nc *nats.Conn) {
				if nc != nil && nc.LastError() != nil {
					logger.Error("nats connection closed; will restart if context allows", "err", nc.LastError())
					return
				}
				logger.Error("nats connection closed; will restart if context allows")
			}),
		)
		if err == nil {
			return nc, nil
		}

		logger.Error("connect to nats failed", "err", err, "retry_in", backoff)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func flushWithTimeout(ctx context.Context, nc *nats.Conn, timeout time.Duration) error {
	if nc == nil || timeout <= 0 {
		return nil
	}
	flushCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return nc.FlushWithContext(flushCtx)
}
