# 01 - Networking

The chat side talks raw TCP. A lot of the code exists because TCP gives you a
stream of bytes, not messages.

## Listening and accepting

```go
ln, err := net.Listen("tcp", ":4000")
```

`net.Listen` returns a `net.Listener`. `Accept` blocks until someone connects and
then hands back a `net.Conn`:

```go
conn, err := ln.Accept()
```

`net.Conn` is an interface with `Read`, `Write`, `Close`, and deadline methods.
If a client writes `hello` and then `world`, the server might read `helloworld`
in one call, or split it somewhere in the middle. Nothing in TCP tells you where
one message ends and the next begins.

## Framing: one command per line

That's why `internal/protocol/frame.go` reads up to a newline with
`bufio.Reader.ReadSlice`. One line is one frame, like `MSG hello there`.

Two limits keep this safe.

First, a maximum size. The reader has a fixed buffer (`BufferSize`). If a line is
longer than that, `ReadSlice` returns `bufio.ErrBufferFull`, which we turn into
`ErrFrameTooLarge` and drop the connection. Without that cap, a client could
send an endless line and make the server allocate until it dies.

Second, a read deadline. Before each read we call
`conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))`. If a client goes quiet
for too long, the read fails and the connection closes. Otherwise idle sockets
pile up forever.

One detail worth knowing: `ReadSlice` returns a slice that points into the
reader's own buffer. It's only valid until the next read. We parse it right away,
and `Parse` copies the pieces into strings, so we never keep the raw slice
around.

## Writing

Writes go through a pooled `bufio.Writer`. `Write` fills an internal buffer and
`Flush` actually sends it. Every write gets a deadline too, so a client that
stops reading can't block a writer forever.

## TLS

`internal/server/server.go` switches to `tls.Listen` when a cert and key are
configured. A TLS connection still satisfies `net.Conn`, so nothing else in the
code changes. We set `MinVersion: tls.VersionTLS12` so old protocol versions are
refused. `make cert` generates a certificate you can use locally.

## TCP keepalive

For plain TCP connections we turn on keepalive:

```go
tc.SetKeepAlive(true)
tc.SetKeepAlivePeriod(30 * time.Second)
```

The OS then sends small probes on idle connections. It helps spot peers that
disappeared without closing the socket properly.

## Graceful shutdown

`Server.Shutdown` does four things:

1. cancels the server context
2. closes the listener, so `Accept` returns
3. closes every live connection, so blocked reads return
4. waits on a `sync.WaitGroup` for the connection goroutines

Step 3 surprised me. Closing the listener is not enough: a goroutine sitting in
`Read` won't notice anything until its connection is closed. Without closing the
connections, shutdown would hang on `WaitGroup.Wait`.
