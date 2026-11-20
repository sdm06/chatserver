package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"example.com/chatserver/internal/config"
	"example.com/chatserver/internal/hub"
)

// Server exposes the HTTP side of the chat server: health, metrics, and a
// live dashboard that streams chat events over Server-Sent Events.
type Server struct {
	hub *hub.Hub
	cfg config.Config
	log *slog.Logger
}

// New creates the web server.
func New(h *hub.Hub, cfg config.Config, log *slog.Logger) *Server {
	return &Server{hub: h, cfg: cfg, log: log}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/metrics", s.metrics)
	mux.HandleFunc("/events", s.events)
	mux.HandleFunc("/", s.dashboard)
	if s.cfg.Dev {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "ok")
}

func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	st := s.hub.Stats()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP chatserver_clients Current connected clients.\n")
	fmt.Fprintf(w, "# TYPE chatserver_clients gauge\n")
	fmt.Fprintf(w, "chatserver_clients %d\n", st.Clients)
	fmt.Fprintf(w, "# HELP chatserver_rooms Current number of rooms.\n")
	fmt.Fprintf(w, "# TYPE chatserver_rooms gauge\n")
	fmt.Fprintf(w, "chatserver_rooms %d\n", st.Rooms)
	fmt.Fprintf(w, "# TYPE chatserver_messages_total counter\n")
	fmt.Fprintf(w, "chatserver_messages_total %d\n", st.Messages)
	fmt.Fprintf(w, "# TYPE chatserver_bytes_total counter\n")
	fmt.Fprintf(w, "chatserver_bytes_total %d\n", st.Bytes)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	ch, ok := s.hub.Subscribe()
	if !ok {
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}
	defer s.hub.Unsubscribe(ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	enc := json.NewEncoder(w)
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case e, open := <-ch:
			if !open {
				return
			}
			fmt.Fprint(w, "data: ")
			if err := enc.Encode(e); err != nil {
				return
			}
			fmt.Fprint(w, "\n")
			flusher.Flush()
		}
	}
}

func (s *Server) dashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>chatserver dashboard</title>
<style>
  body { font-family: ui-monospace, monospace; margin: 2rem; background: #111; color: #eee; }
  h1 { font-size: 1.1rem; }
  #log { border: 1px solid #333; padding: 1rem; height: 60vh; overflow-y: auto; white-space: pre-wrap; }
  .message { color: #9ecbff; }
  .join { color: #8bd17c; }
  .leave { color: #d17c7c; }
</style>
</head>
<body>
<h1>chatserver live dashboard</h1>
<div id="log"></div>
<script>
const log = document.getElementById('log');
const es = new EventSource('/events');
es.onmessage = (m) => {
  const e = JSON.parse(m.data);
  const row = document.createElement('div');
  row.className = e.kind;
  const who = e.nick ? e.nick + ' ' : '';
  row.textContent = '[' + e.room + '] ' + e.kind + ' ' + who + (e.text || '');
  log.appendChild(row);
  log.scrollTop = log.scrollHeight;
};
</script>
</body>
</html>
`
