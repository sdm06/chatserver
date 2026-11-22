package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"example.com/chatserver/internal/bufpool"
	"example.com/chatserver/internal/config"
	"example.com/chatserver/internal/hub"
	"example.com/chatserver/internal/protocol"
	"example.com/chatserver/internal/security"
)

// client is one connection. Two goroutines touch it: readLoop and writeLoop.
// They share only the send/quit channels and the close-once guard.
type client struct {
	conn       net.Conn
	cfg        config.Config
	hub        *hub.Hub
	log        *slog.Logger
	send       chan []byte
	quit       chan struct{}
	closeOnce  sync.Once
	hc         *hub.Client
	id         string
	nick       string
	room       string
	authed     bool
	registered bool
	lim        *security.Limiter
}

func newClient(conn net.Conn, cfg config.Config, h *hub.Hub, log *slog.Logger) *client {
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(30 * time.Second)
	}
	c := &client{
		conn:   conn,
		cfg:    cfg,
		hub:    h,
		log:    log,
		send:   make(chan []byte, 64),
		quit:   make(chan struct{}),
		id:     conn.RemoteAddr().String(),
		room:   "lobby",
		authed: cfg.AuthToken == "",
		lim:    security.NewLimiter(cfg.RatePerSecond, cfg.RateBurst),
	}
	c.hc = &hub.Client{ID: c.id, Room: c.room, Send: c.send, Close: c.Close}
	return c
}

// Close is idempotent: the read loop, write loop, and hub may all call it.
func (c *client) Close() {
	c.closeOnce.Do(func() {
		close(c.quit)
		c.conn.Close()
	})
}

func (c *client) serve(ctx context.Context) {
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		c.writeLoop(ctx)
	}()

	c.readLoop(ctx)
	c.hub.Unregister(c.hc)

	// Ask the writer to stop, but let it flush anything already queued (for
	// example the final "bye" or "invalid token" reply) before we close the
	// socket.
	c.closeOnce.Do(func() { close(c.quit) })
	select {
	case <-writeDone:
	case <-time.After(500 * time.Millisecond):
	}
	c.conn.Close()
}

func (c *client) readLoop(ctx context.Context) {
	r := bufpool.GetReader(c.conn)
	defer bufpool.PutReader(r)

	c.sendf("welcome to chatserver; type HELP for commands\n")
	if !c.authed {
		c.sendf("authentication required: type AUTH <token>\n")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.quit:
			return
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
		line, err := protocol.ReadLine(r, c.cfg.MaxFrame)
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				return
			case errors.Is(err, protocol.ErrFrameTooLarge):
				c.sendf("error: frame too large\n")
				return
			default:
				return
			}
		}
		cmd, err := protocol.Parse(line)
		if err != nil {
			c.sendf("error: " + err.Error() + "\n")
			continue
		}
		if !c.lim.Allow() {
			c.sendf("error: rate limit exceeded, slow down\n")
			continue
		}
		if c.handle(cmd) {
			return
		}
	}
}

// handle returns true when the connection should be closed.
func (c *client) handle(cmd protocol.Command) bool {
	if !c.authed {
		if cmd.Name != "AUTH" {
			c.sendf("error: authentication required (send: AUTH <token>)\n")
			return false
		}
		if len(cmd.Args) != 1 || !security.TokenEqual(cmd.Args[0], c.cfg.AuthToken) {
			c.sendf("error: invalid token\n")
			return true
		}
		c.authed = true
		c.sendf("ok: authenticated\n")
		return false
	}

	switch cmd.Name {
	case "AUTH":
		c.sendf("ok: already authenticated\n")
	case "NICK":
		if c.registered {
			c.sendf("error: nick already set; reconnect to change it\n")
			return false
		}
		if len(cmd.Args) != 1 || !security.ValidNick(cmd.Args[0]) {
			c.sendf("error: invalid nickname (1-32 letters, digits, _ or -)\n")
			return false
		}
		c.nick = cmd.Args[0]
		c.hc.Nick = c.nick
		c.sendf("ok: nick set to " + c.nick + "\n")
		c.register()
	case "JOIN":
		if len(cmd.Args) != 1 || !security.ValidRoom(cmd.Args[0]) {
			c.sendf("error: invalid room (1-32 letters, digits, _ or -)\n")
			return false
		}
		c.changeRoom(cmd.Args[0])
		c.sendf("ok: joined " + cmd.Args[0] + "\n")
	case "MSG":
		if len(cmd.Args) == 0 {
			c.sendf("error: empty message\n")
			return false
		}
		if c.nick == "" {
			c.sendf("error: set a nick first: NICK <name>\n")
			return false
		}
		c.register()
		text := strings.Join(cmd.Args, " ")
		if !security.ValidMessage(text, c.cfg.MaxFrame-64) {
			c.sendf("error: message too long\n")
			return false
		}
		c.hub.Broadcast(hub.Event{
			Time: time.Now(),
			Kind: "message",
			Room: c.room,
			Nick: c.nick,
			Text: text,
		})
	case "WHO":
		st := c.hub.Stats()
		c.sendf(fmt.Sprintf("ok: %d clients in %d rooms\n", st.Clients, st.Rooms))
	case "HELP":
		c.sendf("commands: AUTH <token>, NICK <name>, JOIN <room>, MSG <text>, WHO, QUIT\n")
	case "QUIT":
		c.sendf("bye\n")
		return true
	default:
		c.sendf("error: unknown command " + cmd.Name + "\n")
	}
	return false
}

func (c *client) register() {
	if c.registered || c.nick == "" {
		return
	}
	c.registered = true
	c.hub.Register(c.hc)
}

func (c *client) changeRoom(room string) {
	if c.registered {
		c.hub.Unregister(c.hc)
		c.registered = false
	}
	c.room = room
	c.hc.Room = room
	c.register()
}

// sendf queues text for the writer. It never blocks: if a client is too slow
// to drain its buffer, the message is dropped and the hub eventually evicts it.
func (c *client) sendf(s string) {
	select {
	case c.send <- []byte(s):
	case <-c.quit:
	default:
	}
}

func (c *client) writeLoop(ctx context.Context) {
	w := bufpool.GetWriter(c.conn)
	defer bufpool.PutWriter(w)

	drain := func() {
		for {
			select {
			case msg := <-c.send:
				c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
				if _, err := w.Write(msg); err != nil {
					return
				}
			default:
				_ = w.Flush()
				return
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			drain()
			return
		case <-c.quit:
			drain()
			return
		case msg := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
			if _, err := w.Write(msg); err != nil {
				c.Close()
				return
			}
			if err := w.Flush(); err != nil {
				c.Close()
				return
			}
		}
	}
}
