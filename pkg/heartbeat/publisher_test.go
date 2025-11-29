package heartbeat

import (
	"errors"
	"testing"
)

func TestApplyHostDefaultKeepsProvidedHost(t *testing.T) {
	original := hostname
	defer func() { hostname = original }()
	hostname = func() (string, error) { return "ignored-hostname", nil }

	msg := Message{Host: "explicit"}
	got := applyHostDefault(msg)
	if got.Host != "explicit" {
		t.Fatalf("expected host to remain explicit, got %q", got.Host)
	}
}

func TestApplyHostDefaultUsesHostnameWhenEmpty(t *testing.T) {
	original := hostname
	defer func() { hostname = original }()
	hostname = func() (string, error) { return "local-host", nil }

	got := applyHostDefault(Message{})
	if got.Host != "local-host" {
		t.Fatalf("expected host to default to hostname, got %q", got.Host)
	}
}

func TestApplyHostDefaultIgnoresHostnameErrors(t *testing.T) {
	original := hostname
	defer func() { hostname = original }()
	hostname = func() (string, error) { return "", errors.New("lookup failed") }

	got := applyHostDefault(Message{})
	if got.Host != "" {
		t.Fatalf("expected empty host when lookup fails, got %q", got.Host)
	}
}
