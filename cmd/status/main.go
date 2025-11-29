package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

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

func main() {
	statusURL := flag.String("url", envDefault("STATUS_URL", "http://127.0.0.1:8080/"), "Status endpoint URL")
	timeout := flag.Duration("timeout", envDuration("STATUS_TIMEOUT", 3*time.Second), "HTTP request timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	resp, err := fetchStatus(ctx, *statusURL)
	if err != nil {
		log.Fatalf("fetch status: %v", err)
	}

	printStatus(resp, os.Stdout)
}

func fetchStatus(ctx context.Context, url string) (statusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return statusResponse{}, fmt.Errorf("build request: %w", err)
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return statusResponse{}, fmt.Errorf("request status: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return statusResponse{}, fmt.Errorf("unexpected status %s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	var status statusResponse
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		return statusResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return status, nil
}

func printStatus(resp statusResponse, w io.Writer) {
	if resp.ObservedAt.IsZero() {
		resp.ObservedAt = time.Now()
	}
	fmt.Fprintf(w, "Observed at: %s\n", resp.ObservedAt.Format(time.RFC3339))

	if len(resp.Subjects) == 0 {
		fmt.Fprintln(w, "No heartbeats observed yet.")
		return
	}

	alerting := 0
	fmt.Fprintln(w)

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tSUBJECT\tDESCRIPTION\tHOST\tLAST SEEN\tDETAILS")
	for _, s := range resp.Subjects {
		if s.AlertActive {
			alerting++
		}
		status, details := summarizeSubject(s)

		lastSeen := "-"
		if !s.LastSeen.IsZero() {
			lastSeen = s.LastSeen.Format(time.RFC3339)
		}

		host := s.Host
		if host == "" {
			host = "-"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", status, s.Subject, s.Description, host, lastSeen, details)
	}
	_ = tw.Flush()

	out := buf.String()
	if shouldColor(w) {
		out = colorizeStatuses(out)
	}

	fmt.Fprint(w, out)
	fmt.Fprintf(w, "\n%d alert(s) firing across %d subject(s)\n", alerting, len(resp.Subjects))
}

func summarizeSubject(s subjectState) (string, string) {
	status := "OK"
	details := fmt.Sprintf("interval %s, window %s", s.Interval, s.AllowedWindow)

	if s.AlertActive {
		status = "ALERT!"
		details = fmt.Sprintf("missed %s", fallback(s.MissFor, fmt.Sprintf("past %s", s.AllowedWindow)))
		if s.MissCount > 0 {
			details += fmt.Sprintf(" (%d beats)", s.MissCount)
		}
	} else if s.Missing {
		status = "LATE"
		details = fmt.Sprintf("late by %s", fallback(s.MissFor, s.AllowedWindow))
		if s.MissCount > 0 {
			details += fmt.Sprintf(" (%d beats)", s.MissCount)
		}
	}

	return status, details
}

func fallback(v, defaultVal string) string {
	if strings.TrimSpace(v) == "" {
		return defaultVal
	}
	return v
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

func shouldColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func applyColor(s string, colorize bool, code int) string {
	if !colorize {
		return s
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", code, s)
}

func colorizeStatuses(out string) string {
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if line == "" || strings.HasPrefix(line, "STATUS") {
			continue
		}
		spaceIdx := strings.IndexByte(line, ' ')
		if spaceIdx <= 0 {
			continue
		}
		status := line[:spaceIdx]
		rest := line[spaceIdx:]

		switch status {
		case "ALERT!":
			status = applyColor(status, true, 31)
		case "LATE":
			status = applyColor(status, true, 33)
		case "OK":
			status = applyColor(status, true, 32)
		default:
			// leave as-is
		}
		lines[i] = status + rest
	}
	return strings.Join(lines, "\n")
}
