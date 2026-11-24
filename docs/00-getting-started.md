# 00 - Getting started

A short tour of the project, how to run it, and where to start reading.

It's a TCP chat server with a small HTTP sidecar. Everything uses the Go
standard library, so once Go is installed you can build it offline.

## What you need

- Go 1.27 or newer (`go version`)
- `nc` for manual testing, if you want to poke at it by hand
- Docker, only if you want to build the image

## Layout

```
cmd/chatserver     program entry point (main)
cmd/gencert        writes a local dev TLS certificate
internal/config    settings from environment variables
internal/protocol  line framing and command parsing
internal/bufpool   sync.Pool of bufio readers and writers
internal/hub       rooms, broadcast, history, stats (one goroutine)
internal/server    TCP listener and per-connection loops
internal/security  auth, rate limiting, validation
internal/web       health, metrics, SSE dashboard
docs               these notes
```

The `internal/` prefix means other modules can't import these packages. That's a
sensible default for application code. Each folder is a package: a set of `.go`
files that compile together.

## Run it

```sh
CHAT_AUTH_TOKEN=secret go run ./cmd/chatserver
```

In another terminal:

```sh
nc localhost 4000
```

The first thing the server sends is a welcome line and an auth prompt. Type one
command per line:

```
AUTH secret
NICK alice
MSG hello world
WHO
QUIT
```

If you paste only the token, the server will tell you it wants `AUTH <token>`.
That tripped me up the first time.

Open <http://localhost:8080/> in a browser to watch the same messages arrive on
the dashboard.

## How the program starts

`cmd/chatserver/main.go` has `func main()`. Go runs it, and when it returns the
process exits. Here it does six things:

1. reads config from the environment
2. starts the hub goroutine
3. opens the TCP listener
4. starts the HTTP server
5. waits for Ctrl-C or SIGTERM
6. shuts everything down and waits for connections to finish

## Where to start reading

If you're new to Go, this order worked for me:

1. `internal/protocol/frame.go`: pure functions, easy to test
2. `internal/hub/ring.go`: a small fixed-size data structure
3. `internal/hub/hub.go`: channels and one owner of shared state
4. `internal/server/client.go`: the life of one connection
5. `internal/server/server.go`: accept loop and shutdown
6. `internal/web/web.go`: HTTP handlers
7. `internal/security`: small helpers you can read in one sitting

## Everyday commands

```sh
gofmt -w .                         # format
go vet ./...                       # catch suspicious code
go test ./...                      # run tests
go test -race ./...                # look for data races
go test -bench . -benchmem ./...   # speed and allocations
```

The `Makefile` wraps these, so `make test`, `make race`, and `make bench` work
too.
