package hub

import (
	"context"
	"sync/atomic"
	"time"
)

// Client is the hub's handle on one connected chat client. The hub only reads
// Nick/Room once, at Register time, and never touches them again, which keeps
// the connection goroutine free to mutate its own state.
type Client struct {
	ID    string
	Nick  string
	Room  string
	Send  chan []byte
	Close func()
}

// Event is a single thing that happened in a room. It is rendered as text for
// TCP clients and encoded as JSON for the web dashboard.
type Event struct {
	Time time.Time `json:"time"`
	Kind string    `json:"kind"`
	Room string    `json:"room"`
	Nick string    `json:"nick"`
	Text string    `json:"text"`
}

// Stats is a point-in-time snapshot used by the metrics endpoint.
type Stats struct {
	Clients  int
	Rooms    int
	Messages uint64
	Bytes    uint64
}

type member struct {
	room string
	nick string
}

type statsRequest struct{ reply chan Stats }

type clientRequest struct {
	client *Client
	reply  chan struct{}
}

type broadcastRequest struct {
	event Event
	reply chan struct{}
}

type subRequest struct {
	ch     chan Event
	remove bool
	reply  chan struct{}
}

// Hub owns all shared chat state. A single goroutine (Run) mutates the maps,
// so there are no locks around them: messages arrive over channels instead.
type Hub struct {
	register   chan clientRequest
	unregister chan clientRequest
	broadcast  chan broadcastRequest
	statsReq   chan statsRequest
	subReq     chan subRequest
	done       chan struct{}

	rooms   map[string]map[*Client]struct{}
	members map[*Client]member
	history map[string]*Ring
	subs    map[chan Event]struct{}

	historySize int
	messages    atomic.Uint64
	bytes       atomic.Uint64
}

// New builds a hub that keeps historySize messages per room.
func New(historySize int) *Hub {
	return &Hub{
		register:    make(chan clientRequest),
		unregister:  make(chan clientRequest),
		broadcast:   make(chan broadcastRequest, 256),
		statsReq:    make(chan statsRequest),
		subReq:      make(chan subRequest),
		done:        make(chan struct{}),
		rooms:       make(map[string]map[*Client]struct{}),
		members:     make(map[*Client]member),
		history:     make(map[string]*Ring),
		subs:        make(map[chan Event]struct{}),
		historySize: historySize,
	}
}

// Run processes hub events until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return
		case req := <-h.register:
			h.add(req.client)
			close(req.reply)
		case req := <-h.unregister:
			h.remove(req.client)
			close(req.reply)
		case req := <-h.broadcast:
			h.dispatch(req.event)
			close(req.reply)
		case req := <-h.statsReq:
			req.reply <- h.snapshot()
		case req := <-h.subReq:
			if req.remove {
				if _, ok := h.subs[req.ch]; ok {
					delete(h.subs, req.ch)
					close(req.ch)
				}
			} else {
				h.subs[req.ch] = struct{}{}
			}
			close(req.reply)
		}
	}
}

// Register adds a client to its room and waits until the hub has done so. The
// wait is what guarantees that a NICK followed immediately by MSG is ordered
// correctly. It is safe to call after shutdown.
func (h *Hub) Register(c *Client) {
	reply := make(chan struct{})
	select {
	case h.register <- clientRequest{client: c, reply: reply}:
		<-reply
	case <-h.done:
	}
}

// Unregister removes a client and waits for the hub to finish. Unknown clients
// are ignored, so it is safe to call more than once.
func (h *Hub) Unregister(c *Client) {
	reply := make(chan struct{})
	select {
	case h.unregister <- clientRequest{client: c, reply: reply}:
		<-reply
	case <-h.done:
	}
}

// Broadcast delivers an event to everyone in its room. It waits until the hub
// has fanned the event out, which preserves per-sender ordering.
func (h *Hub) Broadcast(e Event) {
	reply := make(chan struct{})
	select {
	case h.broadcast <- broadcastRequest{event: e, reply: reply}:
		<-reply
	case <-h.done:
	}
}

// Subscribe returns a channel of events for the web dashboard.
func (h *Hub) Subscribe() (chan Event, bool) {
	ch := make(chan Event, 64)
	reply := make(chan struct{})
	select {
	case h.subReq <- subRequest{ch: ch, reply: reply}:
		<-reply
		return ch, true
	case <-h.done:
		return nil, false
	}
}

// Unsubscribe stops delivering events to ch.
func (h *Hub) Unsubscribe(ch chan Event) {
	reply := make(chan struct{})
	select {
	case h.subReq <- subRequest{ch: ch, remove: true, reply: reply}:
		<-reply
	case <-h.done:
	}
}

// Stats returns a consistent snapshot of the hub.
func (h *Hub) Stats() Stats {
	reply := make(chan Stats, 1)
	select {
	case h.statsReq <- statsRequest{reply: reply}:
		return <-reply
	case <-h.done:
		return Stats{}
	}
}

func (h *Hub) add(c *Client) {
	if _, ok := h.members[c]; ok {
		return
	}
	room := c.Room
	if room == "" {
		room = "lobby"
	}
	nick := c.Nick
	if nick == "" {
		nick = "anon"
	}
	h.members[c] = member{room: room, nick: nick}
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]struct{})
	}
	h.rooms[room][c] = struct{}{}
	if h.history[room] == nil {
		h.history[room] = NewRing(h.historySize)
	}
	for _, e := range h.history[room].Snapshot() {
		h.trySend(c, formatEvent(e))
	}
	h.dispatch(Event{Time: time.Now(), Kind: "join", Room: room, Nick: nick})
}

func (h *Hub) remove(c *Client) {
	m, ok := h.members[c]
	if !ok {
		return
	}
	delete(h.members, c)
	if room := h.rooms[m.room]; room != nil {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, m.room)
		}
	}
	h.dispatch(Event{Time: time.Now(), Kind: "leave", Room: m.room, Nick: m.nick})
}

func (h *Hub) dispatch(e Event) {
	text := formatEvent(e)
	if e.Kind == "message" {
		h.messages.Add(1)
		if r := h.history[e.Room]; r != nil {
			r.Push(e)
		}
	}
	h.bytes.Add(uint64(len(text)))

	var slow []*Client
	for c := range h.rooms[e.Room] {
		if !h.trySend(c, text) {
			slow = append(slow, c)
		}
	}
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
	for _, c := range slow {
		if c.Close != nil {
			c.Close()
		}
		h.remove(c)
	}
}

func (h *Hub) trySend(c *Client, b []byte) bool {
	select {
	case c.Send <- b:
		return true
	default:
		return false
	}
}

func (h *Hub) snapshot() Stats {
	clients := 0
	for _, room := range h.rooms {
		clients += len(room)
	}
	return Stats{
		Clients:  clients,
		Rooms:    len(h.rooms),
		Messages: h.messages.Load(),
		Bytes:    h.bytes.Load(),
	}
}

func (h *Hub) shutdown() {
	for _, room := range h.rooms {
		for c := range room {
			if c.Close != nil {
				c.Close()
			}
		}
	}
	for ch := range h.subs {
		close(ch)
	}
	h.rooms = make(map[string]map[*Client]struct{})
	h.members = make(map[*Client]member)
	h.subs = make(map[chan Event]struct{})
}

func formatEvent(e Event) []byte {
	switch e.Kind {
	case "message":
		return []byte("[" + e.Room + "] " + e.Nick + ": " + e.Text + "\n")
	case "join":
		return []byte("[" + e.Room + "] * " + e.Nick + " joined\n")
	case "leave":
		return []byte("[" + e.Room + "] * " + e.Nick + " left\n")
	default:
		return []byte("[" + e.Room + "] * " + e.Text + "\n")
	}
}
