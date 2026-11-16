package hub

// Ring is a fixed-capacity circular buffer. Once full it overwrites the oldest
// entry, so its memory use never grows with traffic. That is exactly what we
// want for chat history: bounded, predictable, allocation-free after creation.
type Ring struct {
	buf   []Event
	start int
	size  int
}

// NewRing creates a ring that holds at most capacity events.
func NewRing(capacity int) *Ring {
	if capacity < 0 {
		capacity = 0
	}
	return &Ring{buf: make([]Event, capacity)}
}

// Push appends e, dropping the oldest event when the ring is full.
func (r *Ring) Push(e Event) {
	if len(r.buf) == 0 {
		return
	}
	if r.size < len(r.buf) {
		r.buf[(r.start+r.size)%len(r.buf)] = e
		r.size++
		return
	}
	r.buf[r.start] = e
	r.start = (r.start + 1) % len(r.buf)
}

// Snapshot returns the events oldest-first in a fresh slice.
func (r *Ring) Snapshot() []Event {
	out := make([]Event, r.size)
	for i := 0; i < r.size; i++ {
		out[i] = r.buf[(r.start+i)%len(r.buf)]
	}
	return out
}

// Len reports how many events are currently stored.
func (r *Ring) Len() int { return r.size }
