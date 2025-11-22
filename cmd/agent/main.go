package main

import (
	"context"
	"flag"
	"fmt"
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
		natsURL   = flag.String("nats-url", envDefault("NATS_URL", nats.DefaultURL), "NATS server URL")
		subject   = flag.String("subject", envDefault("SUBJECT", ""), "Full heartbeat subject (required)")
		interval  = flag.Duration("interval", envDuration("INTERVAL", 15*time.Second), "Heartbeat interval")
		skippable = flag.Int("skippable", envInt("SKIPPABLE", 0), "Number of beats that may be missed before alerting")
		grace     = flag.Duration("grace", envDuration("GRACE", 0), "Optional max duration to miss beats before alerting")
		desc      = flag.String("description", envDefault("DESCRIPTION", ""), "Human-friendly description for alerts")
		debug     = flag.Bool("debug", envBool("DEBUG", false), "Enable debug logging")
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

	nc, err := nats.Connect(*natsURL)
	if err != nil {
		log.Fatalf("connect to nats: %v", err)
	}
	defer nc.Drain()

	pub := heartbeat.NewPublisher(nc, "")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		hb := heartbeat.Message{
			Subject:     *subject,
			GeneratedAt: time.Now().UTC(),
			Interval:    *interval,
			Description: *desc,
		}
		if skippable != nil && *skippable > 0 {
			hb.Skippable = skippable
		}
		if grace != nil && *grace > 0 {
			hb.GracePeriod = grace
		}

		if err := pub.Publish(ctx, hb); err != nil {
			logger.Error("publish heartbeat failed", "err", err, "subject", hb.Subject)
		} else {
			logger.Debug("heartbeat published", "subject", hb.Subject, "interval", hb.Interval, "skippable", hb.Skippable, "grace", hb.GracePeriod)
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

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var parsed int
		if _, err := fmt.Sscanf(v, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}
