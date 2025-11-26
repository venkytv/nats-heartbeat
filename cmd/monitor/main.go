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

	"github.com/venkytv/nats-heartbeat/internal/monitor"
	"github.com/venkytv/nats-heartbeat/internal/notifier"
)

func main() {
	var (
		natsURL     = flag.String("nats-url", envDefault("NATS_URL", nats.DefaultURL), "NATS server URL")
		prefix      = flag.String("subject-prefix", envDefault("SUBJECT_PREFIX", "heartbeat."), "Subject prefix to monitor")
		primeStream = flag.String("prime-stream", envDefault("PRIME_STREAM", ""), "Optional JetStream stream to prime cache from")
		pollEvery   = flag.Duration("poll", envDuration("POLL_INTERVAL", time.Second), "How often to check for missed beats")
		repeatEvery = flag.Duration("repeat-every", envDuration("REPEAT_EVERY", 12*time.Hour), "How often to repeat alerts while beats are missing")
		statusAddr  = flag.String("status-addr", envDefault("STATUS_ADDR", "127.0.0.1:8080"), "Listen address for HTTP status (empty to disable)")
		poUser      = flag.String("pushover-user", os.Getenv("PUSHOVER_USER"), "Pushover user key")
		poToken     = flag.String("pushover-token", os.Getenv("PUSHOVER_TOKEN"), "Pushover app token")
		debug       = flag.Bool("debug", envBool("DEBUG", false), "Enable debug logging")
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

	nc, err := nats.Connect(*natsURL)
	if err != nil {
		log.Fatalf("connect to nats: %v", err)
	}
	defer nc.Drain()

	notify := notifier.Pushover{
		User:  *poUser,
		Token: *poToken,
	}

	cfg := monitor.Config{
		Prefix:      *prefix,
		PrimeStream: *primeStream,
		PollEvery:   *pollEvery,
		RepeatEvery: *repeatEvery,
		StatusAddr:  *statusAddr,
		Debug:       *debug,
		Logger:      logger,
	}
	m := monitor.New(nc, notify, cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := m.Start(ctx); err != nil {
		log.Fatalf("monitor failed: %v", err)
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
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

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || v == "true" || v == "TRUE" || v == "yes" || v == "on"
	}
	return fallback
}
