package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/web"
)

const shutdownTimeout = 10 * time.Second

func main() {
	port := flag.Int("port", web.DefaultPort, "HTTP port to listen on")
	dataDir := flag.String("data-dir", web.DefaultDataDir, "directory holding training runs (weights and metrics)")
	staticDir := flag.String("static-dir", web.DefaultStaticDir, "directory holding static web assets")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := web.EnsureDataDir(*dataDir); err != nil {
		logger.Error("failed to prepare data directory", "data_dir", *dataDir, "error", err)
		os.Exit(1)
	}

	server := web.NewServer(web.Config{
		Port:      *port,
		DataDir:   *dataDir,
		StaticDir: *staticDir,
		Logger:    logger,
	})

	if err := server.Listen(); err != nil {
		logger.Error("failed to bind address", "port", *port, "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- server.Start() }()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}
