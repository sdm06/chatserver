# 02 - Concurrency

Each connection gets two goroutines and a couple of channels. That's most of the
concurrency in the project.

## One goroutine per connection

`server.Serve` starts a goroutine for every accepted connection:

```go
go func(conn net.Conn) {
    c := newClient(conn, ...)
    c.serve(ctx)
}(conn)
```

Inside `serve`, a client runs two loops. `readLoop` reads commands and `writeLoop`
writes replies. They don't share mutable state. They pass bytes through a
channel, which is the whole idea behind the Go proverb:

> Do not communicate by sharing memory; share memory by communicating.

## Channels as queues

The client has two channels:

```go
send chan []byte    // outgoing text
quit chan struct{}  // closed to say "stop"
```

`readLoop` and the hub put bytes on `send`, and `writeLoop` takes them off and
writes them to the socket. Closing `quit` tells both loops to stop.

`sendf` uses a `select` with a `default`, so it never blocks:

```go
select {
case c.send <- []byte(s):
case <-c.quit:
default: // buffer is full, drop it
}
```

Dropping is on purpose. One slow client shouldn't be able to stall a busy room.

## The hub owns the shared state

Rooms, members, and history are maps, and maps aren't safe for concurrent writes.
Rather than locking them, one goroutine (`Hub.Run`) owns them:

```go
for {
    select {
    case <-ctx.Done():          // shutdown
    case req := <-h.register:   // add a member
    case req := <-h.unregister:
    case req := <-h.broadcast:
    case req := <-h.statsReq:
    case req := <-h.subReq:
    }
}
```

Every other goroutine talks to the hub over channels. Since only `Run` touches
the maps, there's no mutex and no data race. `go test -race` confirms it.

`Register`, `Unregister`, and `Broadcast` send a request and wait for a reply.
That wait matters. My first version just dropped the request on a channel and
returned, and it was subtly wrong: a client that sent `NICK`, `MSG`, then `QUIT`
quickly could have the hub process the unregister before the broadcast, and the
message would disappear. Waiting for the reply fixes the ordering. It costs one
channel round-trip per call, which is fine here.

`Run` closes `h.done` when it exits. The public methods also select on `done`, so
a late call after shutdown returns instead of blocking forever.

## Fan-out and backpressure

`dispatch` sends the formatted text to everyone in the room with a non-blocking
send. If a client's buffer is full, it gets collected and removed:

```go
if !h.trySend(c, text) {
    slow = append(slow, c)
}
// ...
for _, c := range slow {
    c.Close()
    h.remove(c)
}
```

That's backpressure. The server protects itself instead of letting queues grow
without limit.

## Contexts

`context.Context` carries cancellation. The server cancels its context on
shutdown, and loops check `ctx.Done()` so they can exit. The web handlers use
`r.Context()`, which is cancelled when the browser disconnects, so SSE streams
stop on their own.

## WaitGroup

`sync.WaitGroup` counts in-flight goroutines. Shutdown waits on it so the process
doesn't exit while a write is half done.

## Mistakes I ran into

- Starting a goroutine with no way to stop it. It leaks.
- Closing a channel from two places. Here `Close` is guarded by `sync.Once`.
- Sharing a map between goroutines without a lock. Here only the hub goroutine
  touches the maps.
- Blocking on a send inside the broadcast loop. A single stuck client would stop
  the room, so those sends are non-blocking.
