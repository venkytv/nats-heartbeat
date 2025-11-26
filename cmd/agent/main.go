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
		natsURL  = flag.String("nats-url", envDefault("NATS_URL", nats.DefaultURL), "NATS server URL")
		subject  = flag.String("subject", envDefault("SUBJECT", ""), "Full heartbeat subject (required)")
		interval = flag.Duration("interval", envDuration("INTERVAL", 15*time.Second), "Heartbeat interval")
		grace    = flag.Duration("grace", envDuration("GRACE", 0), "Optional max duration to miss beats before alerting")
		desc     = flag.String("description", envDefault("DESCRIPTION", ""), "Human-friendly description for alerts")
		debug    = flag.Bool("debug", envBool("DEBUG", false), "Enable debug logging")
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
			nats.ClosedHandler(func(_ *nats.Conn) {
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
