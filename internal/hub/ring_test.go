package hub

import "testing"

func TestRingWrap(t *testing.T) {
	r := NewRing(3)
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		r.Push(Event{Text: s})
	}
	got := r.Snapshot()
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	want := []string{"c", "d", "e"}
	for i, s := range want {
		if got[i].Text != s {
			t.Fatalf("at %d got %q want %q", i, got[i].Text, s)
		}
	}
}

func TestRingZeroCapacity(t *testing.T) {
	r := NewRing(0)
	r.Push(Event{Text: "ignored"})
	if r.Len() != 0 {
		t.Fatalf("len = %d", r.Len())
	}
}

func BenchmarkRingPush(b *testing.B) {
	r := NewRing(64)
	e := Event{Kind: "message", Room: "lobby", Nick: "alice", Text: "hello"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Push(e)
	}
}
