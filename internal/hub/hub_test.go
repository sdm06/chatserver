package hub

import (
	"context"
	"strings"
	"testing"
	"time"
)

func startHub(t *testing.T) *Hub {
	t.Helper()
	h := New(2)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go h.Run(ctx)
	return h
}

func recv(t *testing.T, ch chan []byte) string {
	t.Helper()
	select {
	case b := <-ch:
		return string(b)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
		return ""
	}
}

func TestHubBroadcast(t *testing.T) {
	h := startHub(t)
	send := make(chan []byte, 8)
	c := &Client{ID: "1", Nick: "alice", Room: "lobby", Send: send}
	h.Register(c)

	if got := recv(t, send); !strings.Contains(got, "alice joined") {
		t.Fatalf("join = %q", got)
	}
	h.Broadcast(Event{Kind: "message", Room: "lobby", Nick: "alice", Text: "hi"})
	if got := recv(t, send); !strings.Contains(got, "[lobby] alice: hi") {
		t.Fatalf("message = %q", got)
	}
}

func TestHubHistoryReplay(t *testing.T) {
	h := startHub(t)
	first := make(chan []byte, 8)
	h.Register(&Client{ID: "1", Nick: "alice", Room: "lobby", Send: first})
	recv(t, first) // join
	h.Broadcast(Event{Kind: "message", Room: "lobby", Nick: "alice", Text: "hello"})
	recv(t, first) // message, guarantees history was written

	second := make(chan []byte, 8)
	h.Register(&Client{ID: "2", Nick: "bob", Room: "lobby", Send: second})
	if got := recv(t, second); !strings.Contains(got, "[lobby] alice: hello") {
		t.Fatalf("history = %q", got)
	}
}

func TestHubSubscribe(t *testing.T) {
	h := startHub(t)
	ch, ok := h.Subscribe()
	if !ok {
		t.Fatal("subscribe failed")
	}
	h.Broadcast(Event{Kind: "message", Room: "lobby", Nick: "alice", Text: "hi"})
	select {
	case e := <-ch:
		if e.Text != "hi" {
			t.Fatalf("text = %q", e.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	h.Unsubscribe(ch)
	if _, open := <-ch; open {
		t.Fatal("channel should be closed after unsubscribe")
	}
}

func TestHubSlowClientEvicted(t *testing.T) {
	h := startHub(t)
	send := make(chan []byte) // unbuffered, never drained
	closed := make(chan struct{})
	c := &Client{ID: "1", Nick: "slow", Room: "lobby", Send: send, Close: func() { close(closed) }}
	h.Register(c)
	// The join dispatch fills nothing (unbuffered, no reader) and evicts c.
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("slow client was not closed")
	}
}
