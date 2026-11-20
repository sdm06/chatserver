package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/chatserver/internal/config"
	"example.com/chatserver/internal/hub"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := hub.New(4)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go h.Run(ctx)

	s := New(h, config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func getBody(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func TestHealth(t *testing.T) {
	ts := newTestServer(t)
	status, body := getBody(t, ts.URL+"/healthz")
	if status != http.StatusOK || !strings.Contains(body, "ok") {
		t.Fatalf("status=%d body=%q", status, body)
	}
}

func TestMetrics(t *testing.T) {
	ts := newTestServer(t)
	status, body := getBody(t, ts.URL+"/metrics")
	if status != http.StatusOK || !strings.Contains(body, "chatserver_clients") {
		t.Fatalf("status=%d body=%q", status, body)
	}
}

func TestDashboard(t *testing.T) {
	ts := newTestServer(t)
	status, body := getBody(t, ts.URL+"/")
	if status != http.StatusOK || !strings.Contains(body, "EventSource") {
		t.Fatalf("status=%d body=%q", status, body)
	}
}
