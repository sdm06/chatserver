# 04 - Security

The assumption here is that clients are hostile. None of this is exotic, but
it's the minimum I'd want before putting a socket on the internet.

## What can go wrong

- Someone sends a gigantic line and exhausts memory.
- Someone floods messages and burns CPU or bandwidth.
- Someone guesses the auth token by timing the comparison.
- Someone opens thousands of connections.
- Someone sniffs the traffic.
- A weird message breaks the dashboard.

Each one has a matching defense in the code.

## TLS

Set `CHAT_TLS_CERT` and `CHAT_TLS_KEY` to serve over TLS. We force
`MinVersion: tls.VersionTLS12`, so SSLv3 and TLS 1.0/1.1 are refused.

`make cert` generates a self-signed cert for local testing. Don't ship that to
production. Use a real CA.

## Token auth

When `CHAT_AUTH_TOKEN` is set, a client has to send `AUTH <token>` before it can
do anything else. The comparison uses `crypto/subtle`:

```go
return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
```

A normal `==` stops at the first byte that differs. If an attacker can measure
timing, that leaks information one byte at a time. Constant-time comparison looks
at every byte.

A couple of notes:

- It's a shared secret, so hand it out over a separate channel.
- Send it over TLS, or it's visible on the wire.
- For real users, replace it with per-user credentials or signed tokens.

## Rate limiting

Each client gets a token bucket in `internal/security/ratelimit.go`:

- it starts with `burst` tokens
- each command spends one
- tokens refill at `rate` per second, up to `burst`

So a client can send a quick burst, then settles into a steady rate. Anything
over the limit gets `error: rate limit exceeded`.

This keeps normal bursts working while capping abuse.

## Input validation

`internal/security/validate.go` checks:

- nicknames and rooms: 1-32 characters, letters, digits, `_` and `-` only
- messages: length capped, NUL bytes rejected

The frame size cap already limits the bytes, so why validate the characters?
Because these values end up in formatted output and JSON. Restricting the
character set keeps control characters out of terminals and logs.

## Limits everywhere

- `CHAT_MAX_FRAME` caps each line, with a hard maximum of 4096 bytes.
- The `bufio` reader is fixed-size, so oversized input fails fast.
- `CHAT_MAX_CLIENTS` caps concurrent connections.
- The outgoing buffer per client is bounded at 64 messages. Slow clients get
  dropped.
- History per room is a fixed-size ring, sized by `CHAT_HISTORY`.

Every queue has a ceiling. An unbounded queue is a denial-of-service bug waiting
for traffic.

## Timeouts

- Read deadline: idle clients get disconnected.
- Write deadline: a client that stops reading can't block a writer forever.
- HTTP `ReadHeaderTimeout`: limits slow header attacks.
- Shutdown timeout: the process won't hang forever on exit.

## The container

The `Dockerfile` builds with `CGO_ENABLED=0`, runs on distroless `static:nonroot`,
runs as a non-root user, and ships only the binary. No shell, no package manager.
Less in the image means less to attack.

## Secrets

Config comes from environment variables, so secrets aren't baked into the binary.
Don't log them. In a real deployment, use your platform's secret manager instead
of plain env vars.

## What this doesn't do

- No password hashing. The token is a pre-shared secret, not a user password. If
  you add accounts, hash passwords with argon2 or bcrypt.
- No persistence. Restart and the history is gone.
- No encryption beyond TLS.

The shared token is the part I'd change first for real use. Everyone knowing the
same secret means you can't revoke one person without rotating it for everyone.
