package server

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"example.com/chatserver/internal/config"
	"example.com/chatserver/internal/hub"
)

// Server owns the TCP listener and the pool of connection goroutines.
type Server struct {
	cfg      config.Config
	hub      *hub.Hub
	log      *slog.Logger
	listener net.Listener
	clients  atomic.Int64
	wg       sync.WaitGroup
	conns    sync.Map
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a server. Call Listen, then Serve.
func New(cfg config.Config, h *hub.Hub, log *slog.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{cfg: cfg, hub: h, log: log, ctx: ctx, cancel: cancel}
}

// Listen opens the TCP (or TLS) listener.
func (s *Server) Listen() error {
	var (
		ln  net.Listener
		err error
	)
	if s.cfg.TLSEnabled() {
		var cert tls.Certificate
		cert, err = tls.LoadX509KeyPair(s.cfg.TLSCert, s.cfg.TLSKey)
		if err != nil {
			return err
		}
		ln, err = tls.Listen("tcp", s.cfg.TCPAddr, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})
	} else {
		ln, err = net.Listen("tcp", s.cfg.TCPAddr)
	}
	if err != nil {
		return err
	}
	s.listener = ln
	return nil
}

// Addr returns the address the server is bound to (useful when TCPAddr is :0).
func (s *Server) Addr() net.Addr {
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

// Serve accepts connections until Shutdown is called.
func (s *Server) Serve() error {
	if s.listener == nil {
		return errors.New("server: Listen must be called before Serve")
	}
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		if int(s.clients.Load()) >= s.cfg.MaxClients {
			conn.Close()
			s.log.Warn("rejected connection: max clients reached", "remote", conn.RemoteAddr())
			continue
		}
		s.clients.Add(1)
		s.wg.Add(1)
		s.conns.Store(conn, struct{}{})
		go func(conn net.Conn) {
			defer s.wg.Done()
			defer s.clients.Add(-1)
			defer s.conns.Delete(conn)
			c := newClient(conn, s.cfg, s.hub, s.log)
			c.serve(s.ctx)
		}(conn)
	}
}

// Shutdown stops accepting, closes live connections, and waits for goroutines.
func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel()
	var err error
	if s.listener != nil {
		err = s.listener.Close()
	}
	s.conns.Range(func(k, _ any) bool {
		if conn, ok := k.(net.Conn); ok {
			conn.Close()
		}
		return true
	})
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return err
}
