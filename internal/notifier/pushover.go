package notifier

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Pushover sends notifications via the pushover API.
type Pushover struct {
	Token    string
	User     string
	Endpoint string
	Client   *http.Client
}

func (p Pushover) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p Pushover) Alert(ctx context.Context, evt Event) error {
	return p.send(ctx, "Heartbeat missed", fmt.Sprintf("%s: missed %d beats over %s (interval %s)", evt.Description, evt.MissCount, evt.MissFor, evt.Interval))
}

func (p Pushover) Resolved(ctx context.Context, evt Event) error {
	return p.send(ctx, "Heartbeat resolved", fmt.Sprintf("%s: recovered at %s", evt.Description, evt.LastSeen.UTC().Format(time.RFC3339)))
}

func (p Pushover) send(ctx context.Context, title, message string) error {
	if p.Token == "" || p.User == "" {
		return errors.New("pushover token and user are required")
	}
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = "https://api.pushover.net/1/messages.json"
	}
	data := url.Values{}
	data.Set("token", p.Token)
	data.Set("user", p.User)
	data.Set("title", title)
	data.Set("message", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("pushover returned status %s", resp.Status)
	}
	return nil
}
