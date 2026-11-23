package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"example.com/chatserver/internal/config"
	"example.com/chatserver/internal/hub"
)

func testConfig() config.Config {
	return config.Config{
		TCPAddr:       "127.0.0.1:0",
		MaxFrame:      4096,
		MaxClients:    10,
		HistorySize:   10,
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  2 * time.Second,
		RatePerSecond: 1000,
		RateBurst:     1000,
	}
}

func startServer(t *testing.T, cfg config.Config) *Server {
	t.Helper()
	h := hub.New(cfg.HistorySize)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go h.Run(ctx)

	srv := New(cfg, h, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() {
		shCtx, shCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shCancel()
		_ = srv.Shutdown(shCtx)
	})
	return srv
}

func dial(t *testing.T, srv *Server) (net.Conn, func(string)) {
	t.Helper()
	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	sc := bufio.NewScanner(conn)
	expect := func(sub string) {
		t.Helper()
		for sc.Scan() {
			if strings.Contains(sc.Text(), sub) {
				return
			}
		}
		t.Fatalf("expected %q, scan err: %v", sub, sc.Err())
	}
	return conn, expect
}

func TestChatFlow(t *testing.T) {
	cfg := testConfig()
	cfg.AuthToken = "secret"
	srv := startServer(t, cfg)
	conn, expect := dial(t, srv)

	expect("welcome")
	expect("authentication required")

	fmt.Fprintln(conn, "MSG hello")
	expect("authentication required")

	fmt.Fprintln(conn, "AUTH secret")
	expect("authenticated")

	fmt.Fprintln(conn, "NICK alice")
	expect("nick set to alice")
	expect("alice joined")

	fmt.Fprintln(conn, "MSG hello everyone")
	expect("[lobby] alice: hello everyone")

	fmt.Fprintln(conn, "WHO")
	expect("1 clients in 1 rooms")
}

func TestAuthBadToken(t *testing.T) {
	cfg := testConfig()
	cfg.AuthToken = "secret"
	srv := startServer(t, cfg)
	conn, expect := dial(t, srv)

	expect("welcome")
	fmt.Fprintln(conn, "AUTH nope")
	expect("invalid token")
}

func TestTwoClients(t *testing.T) {
	cfg := testConfig()
	srv := startServer(t, cfg)
	c1, e1 := dial(t, srv)
	c2, e2 := dial(t, srv)

	e1("welcome")
	e2("welcome")

	fmt.Fprintln(c1, "NICK alice")
	e1("alice joined")

	fmt.Fprintln(c2, "NICK bob")
	e2("bob joined")
	e1("bob joined")

	fmt.Fprintln(c1, "MSG hi bob")
	e1("[lobby] alice: hi bob")
	e2("[lobby] alice: hi bob")
}
