# 05 - Memory efficiency

Memory efficiency here means three things: reuse allocations where you can, put
a ceiling on every queue, and measure instead of guessing.

## Reusing buffers

Every connection needs a `bufio.Reader` and a `bufio.Writer`, each with a 4 KiB
buffer. Allocating them per connection is fine at low load but creates a lot of
garbage when connections churn.

`internal/bufpool` keeps them in a `sync.Pool`:

```go
br := readers.Get().(*bufio.Reader)
br.Reset(conn)
// ... use it ...
br.Reset(emptyReader) // drop the connection reference
readers.Put(br)
```

Two things matter here. First, resetting before `Put` drops the reference to the
connection, so the pool doesn't keep a dead socket alive. Second, `sync.Pool` is
a cache, not a guarantee. The runtime can drop pooled objects during GC. That's
fine; the `New` function creates new ones when needed.

## A fixed-size history ring

A naive history is a slice that grows forever. `internal/hub/ring.go` allocates
exactly `capacity` slots once and wraps around:

```go
buf[(start+size)%len(buf)] = e
```

When it's full, the oldest event gets overwritten. Memory stays constant no
matter how many messages flow through. `CHAT_HISTORY` sets the capacity.

## Bounded channels

Channels are queues, and an unbounded queue is a memory leak with extra steps.
Every channel here has a size or a drop policy:

- per-client `send` buffer: 64 messages, then messages are dropped
- hub broadcast requests: buffered at 256, and senders wait for fan-out so
  ordering stays correct
- SSE subscriber buffers: 64 events, then events are dropped

A slow consumer can't make the server allocate without limit.

## Zero-copy reads

`bufio.Reader.ReadSlice` returns a slice that aliases the reader's internal
buffer. It doesn't allocate per line. We parse it immediately, and `Parse` copies
only the command name and arguments. The raw slice is never stored.

## Preallocate when you know the size

`Ring.Snapshot` allocates exactly `size` events with `make([]Event, r.size)`.
That's one allocation instead of letting `append` grow a slice several times.

## No goroutine per message

Some designs start a goroutine per message. That's expensive: every goroutine has
a stack and has to be scheduled. Here messages go through channels to a fixed set
of goroutines: one reader and one writer per connection, plus the hub.

## Measure it

Benchmarks live next to the code and report allocations:

```sh
make bench
# or
go test -run '^$' -bench . -benchmem ./...
```

`-benchmem` prints `B/op` and `allocs/op`. It's worth watching those numbers when
you change something. A change that's faster but allocates ten times more is
usually a regression for a server.

These are the numbers from my machine:

```
BenchmarkGetPutReader-8   37.82 ns/op    0 B/op   0 allocs/op
BenchmarkGetPutWriter-8   22.28 ns/op    0 B/op   0 allocs/op
BenchmarkRingPush-8       15.63 ns/op    0 B/op   0 allocs/op
BenchmarkReadLine-8       1240 ns/op   4144 B/op   2 allocs/op
BenchmarkParse-8          743.7 ns/op   176 B/op    2 allocs/op
```

The ring push and the pool get/put calls are allocation-free, which is what we
want. `ReadLine` allocates because the benchmark builds a fresh `bufio.Reader`
every iteration; in the server the reader comes from the pool. `Parse` allocates
because it copies the command and arguments into strings. Your numbers will
differ, so run it yourself.

## Profiling

Go ships with `net/http/pprof`. Start with `CHAT_DEV=true` and the web sidecar
mounts it at <http://localhost:8080/debug/pprof/>. The useful views:

- `heap`: what's using memory right now
- `allocs`: what's been allocated over time
- `goroutine`: whether goroutines are leaking

It's disabled by default so it doesn't end up on a public endpoint.

## Let the GC do its job

The garbage collector is good, but it can only reclaim what you stop
referencing. The biggest wins in server code usually come from three places:
fewer allocations per message, bounded buffers, and no goroutine leaks. A leaked
goroutine pins its stack and everything it points at, so a small leak can hold a
surprising amount of memory.

That's what this project is trying to do.
