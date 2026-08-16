package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	DefaultPort    = 8080
	DefaultDataDir = "./data"
)

type Config struct {
	Port      int
	DataDir   string
	StaticDir string
	Logger    *slog.Logger
}

type Server struct {
	dataDir string
	logger  *slog.Logger
	http    *http.Server

	mu       sync.Mutex
	listener net.Listener
}

func NewServer(cfg Config) *Server {
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}
	if cfg.StaticDir == "" {
		cfg.StaticDir = DefaultStaticDir
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	s := &Server{
		dataDir: cfg.DataDir,
		logger:  cfg.Logger,
	}
	s.http = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           s.routes(cfg.StaticDir),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) routes(staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/runs", s.handleRuns)
	mux.Handle("/", staticHandler(staticDir, s.logger))
	return mux
}

func (s *Server) Handler() http.Handler { return s.http.Handler }

func (s *Server) Listen() error {
	ln, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()
	return nil
}

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return s.http.Addr
	}
	return s.listener.Addr().String()
}

func (s *Server) Start() error {
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()

	if ln == nil {
		if err := s.Listen(); err != nil {
			return err
		}
		s.mu.Lock()
		ln = s.listener
		s.mu.Unlock()
	}

	s.logger.Info("server listening", "addr", ln.Addr().String(), "data_dir", s.dataDir)

	if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down")
	return s.http.Shutdown(ctx)
}
