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

	"github.com/JoaoVictorVM/2048RL/internal/agent"
	"github.com/JoaoVictorVM/2048RL/internal/game"
)

const (
	DefaultPort    = 8080
	DefaultDataDir = "./data"
)

type Config struct {
	Port       int
	DataDir    string
	StaticDir  string
	SessionTTL time.Duration
	NewGame    func() *game.Game
	// AgentConfig define a configuração de tuplas usada para carregar
	// checkpoints; o valor zero usa agent.DefaultConfig.
	AgentConfig agent.Config
	Logger      *slog.Logger
}

type Server struct {
	dataDir  string
	logger   *slog.Logger
	http     *http.Server
	sessions *SessionStore
	newGame  func() *game.Game
	agentCfg agent.Config
	live     livePacing

	mu        sync.Mutex
	listener  net.Listener
	stopSweep chan struct{}
	sweepOnce sync.Once
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
	if cfg.NewGame == nil {
		cfg.NewGame = func() *game.Game { return game.NewGame() }
	}
	if len(cfg.AgentConfig.Tuples) == 0 {
		cfg.AgentConfig = agent.DefaultConfig()
	}

	s := &Server{
		dataDir:   cfg.DataDir,
		logger:    cfg.Logger,
		sessions:  NewSessionStore(cfg.SessionTTL),
		newGame:   cfg.NewGame,
		agentCfg:  cfg.AgentConfig,
		live:      livePacing{moveDelay: DefaultMoveDelay, restartDelay: DefaultEpisodeRestartDelay},
		stopSweep: make(chan struct{}),
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
	mux.HandleFunc("POST /api/human/new", s.handleHumanNew)
	mux.HandleFunc("POST /api/human/move", s.handleHumanMove)
	mux.HandleFunc("GET /api/human/reference", s.handleHumanReference)
	mux.HandleFunc("GET /ws/live", s.handleLiveWS)
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

	go s.sweepSessions()

	if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) sweepSessions() {
	ticker := time.NewTicker(sessionSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopSweep:
			return
		case <-ticker.C:
			if removed := s.sessions.Sweep(); removed > 0 {
				s.logger.Info("expired human play sessions removed", "count", removed)
			}
		}
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down")
	s.sweepOnce.Do(func() { close(s.stopSweep) })
	return s.http.Shutdown(ctx)
}
