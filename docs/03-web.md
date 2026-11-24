# 03 - Web

The HTTP side is small on purpose. It's there to show that the same events that
go to TCP clients can also feed a browser.

Everything lives in `internal/web/web.go` and uses `net/http` only.

## Handlers

A handler is a function with this shape:

```go
func(w http.ResponseWriter, r *http.Request)
```

`http.ServeMux` maps paths to handlers:

```go
mux.HandleFunc("/healthz", s.health)
mux.HandleFunc("/metrics", s.metrics)
mux.HandleFunc("/events", s.events)
mux.HandleFunc("/", s.dashboard)
```

The `/` pattern is the catch-all, so it serves the dashboard for anything that
doesn't match a more specific path. Set `Content-Type` before you write the body.

## Health check

`/healthz` returns `200 ok`. Load balancers and orchestrators poll endpoints like
this to decide whether the process is alive. Keep them cheap and free of
dependencies.

## Metrics

`/metrics` prints a Prometheus-compatible text format:

```
# TYPE chatserver_clients gauge
chatserver_clients 3
# TYPE chatserver_messages_total counter
chatserver_messages_total 42
```

The values come from `hub.Stats()`, which asks the hub goroutine for a consistent
snapshot. `Messages` and `Bytes` are `atomic.Uint64` counters that get bumped
during dispatch.

## Server-Sent Events

WebSockets need extra libraries and handshake code. SSE is just an HTTP response
that stays open and keeps sending lines like this:

```
data: {"kind":"message","room":"lobby","nick":"alice","text":"hi"}

```

The browser reads it with the built-in `EventSource` API, so no JavaScript
library is needed.

On the server:

1. Check that the `ResponseWriter` supports flushing (`http.Flusher`).
2. Subscribe to the hub to get a channel of events.
3. Set the SSE headers and flush right away, so the browser knows the stream is
   live.
4. For each event, write `data: `, encode the event as JSON, write a blank line,
   and flush.
5. Send a `: ping` comment every 20 seconds so proxies don't time the stream out.
6. Stop when `r.Context()` is done or the hub closes the channel.

A few gotchas:

- `json.Encoder.Encode` already adds a newline, so you need one more for the blank
  line between events.
- `Flush` is what actually pushes bytes to the client. Without it everything sits
  in a buffer.
- If a subscriber's channel is full, the hub drops events for that subscriber
  instead of blocking the whole server.

## Dashboard

`/` serves a tiny HTML page. Its script opens `new EventSource('/events')` and
appends each event to the page. That's all it does. The point is to show the
events flowing, not to build a polished UI.

## pprof in dev

Start the server with `CHAT_DEV=true` and the profiler is mounted at
<http://localhost:8080/debug/pprof/>. It's off by default because you don't want
that exposed in production.

## Serving HTTP safely

In `cmd/chatserver/main.go` the HTTP server sets:

```go
ReadHeaderTimeout: 5 * time.Second
```

That limits slowloris-style attacks, where a client sends headers a byte at a
time. Production setups often set `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`
as well.
