package security

import (
	"testing"
	"time"
)

func TestLimiterBurst(t *testing.T) {
	l := NewLimiter(1, 3)
	for i := 0; i < 3; i++ {
		if !l.Allow() {
			t.Fatalf("call %d should be allowed", i)
		}
	}
	if l.Allow() {
		t.Fatal("expected denial after burst exhausted")
	}
}

func TestLimiterRefill(t *testing.T) {
	l := NewLimiter(1, 1)
	now := time.Unix(0, 0)
	l.now = func() time.Time { return now }
	l.last = now

	if !l.Allow() {
		t.Fatal("first call should be allowed")
	}
	if l.Allow() {
		t.Fatal("second call should be denied")
	}
	now = now.Add(time.Second)
	if !l.Allow() {
		t.Fatal("should be allowed after refill")
	}
}
