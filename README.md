# chatserver

A small TCP chat server in Go, with a web dashboard next to it. No dependencies,
just the standard library.

I wrote this to practice the parts of Go I kept reading about but hadn't used
together: raw TCP, framing, goroutines and channels, TLS, auth, rate limiting,
and keeping allocations down. The code is meant to be read, and the `docs/`
folder walks through each piece.

## Run it

```sh
# terminal 1
CHAT_AUTH_TOKEN=secret make run

# terminal 2
nc localhost 4000
AUTH secret
NICK alice
MSG hello world
QUIT
```

Each line you type in `nc` is a command. If you send just `secret`, the server
will ask for `AUTH secret`.

Then open <http://localhost:8080/> to watch messages show up live.

## What's in here

- TCP server with line-based framing and a hard size limit per line
- rooms, broadcast, and a fixed-size history per room
- token auth, per-client rate limiting, and input validation
- TLS support, plus a small helper to generate a local cert
- HTTP sidecar with health, metrics, and an SSE dashboard
- pooled buffers and bounded queues, with benchmarks to show the cost

## Commands

| Command | What it does |
|---|---|
| `make run` | Run the server |
| `make test` | Unit and integration tests |
| `make race` | Tests with the race detector |
| `make bench` | Benchmarks with allocation stats |
| `make vet` | Static analysis |
| `make fmt` | Format all Go files |
| `make cert` | Generate a local self-signed TLS cert |
| `make docker` | Build the container image |

## Protocol

One command per line, arguments separated by spaces:

```
AUTH <token>      authenticate (required when CHAT_AUTH_TOKEN is set)
NICK <name>       set a nickname (1-32 letters, digits, _ or -)
JOIN <room>       join a room (default: lobby)
MSG <text...>     send a message to the current room
WHO               show client/room counts
HELP              list commands
QUIT              disconnect
```

## Configuration

Everything is set through environment variables.

| Variable | Default | Meaning |
|---|---|---|
| `CHAT_TCP_ADDR` | `:4000` | TCP listen address |
| `CHAT_HTTP_ADDR` | `:8080` | HTTP listen address |
| `CHAT_AUTH_TOKEN` | empty | Shared token; empty disables auth |
| `CHAT_TLS_CERT` / `CHAT_TLS_KEY` | empty | Enable TLS when both set |
| `CHAT_MAX_FRAME` | `4096` | Max bytes per line (clamped 512-4096) |
| `CHAT_MAX_CLIENTS` | `1000` | Max concurrent connections |
| `CHAT_HISTORY` | `50` | Messages replayed per room |
| `CHAT_READ_TIMEOUT` | `10m` | Idle read deadline |
| `CHAT_WRITE_TIMEOUT` | `10s` | Per-write deadline |
| `CHAT_RATE` / `CHAT_RATE_BURST` | `5` / `10` | Per-client rate limit |
| `CHAT_DEV` | `false` | Silence the auth warning and mount pprof |

## TLS

```sh
make cert
CHAT_TLS_CERT=cert.pem CHAT_TLS_KEY=key.pem CHAT_AUTH_TOKEN=secret make run
openssl s_client -connect localhost:4000 -quiet
```

The generated cert is for local testing only. Use a real CA in production.

## Docs

- [Getting started](docs/00-getting-started.md)
- [Networking](docs/01-networking.md)
- [Concurrency](docs/02-concurrency.md)
- [Web](docs/03-web.md)
- [Security](docs/04-security.md)
- [Memory efficiency](docs/05-memory-efficiency.md)

## If I keep going

The obvious next steps are persistence (SQLite or Redis), real user accounts
with hashed passwords, and a proper metrics dashboard instead of the raw text
endpoint. A WebSocket transport would also be nice for the browser, since SSE
only goes one way.
