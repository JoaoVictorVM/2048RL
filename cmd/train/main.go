package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/JoaoVictorVM/2048RL/internal/agent"
	"github.com/JoaoVictorVM/2048RL/internal/train"
)

func main() {
	episodes := flag.Int("episodes", 0, "número de episódios de self-play (obrigatório)")
	learningRate := flag.Float64("learning-rate", train.DefaultLearningRate, "taxa de aprendizado do update TD")
	checkpointInterval := flag.Int("checkpoint-interval", train.DefaultCheckpointInterval, "intervalo de episódios entre checkpoints")
	logInterval := flag.Int("log-interval", train.DefaultLogInterval, "intervalo de episódios entre relatórios no console")
	runID := flag.String("run-id", "", "identificador do run (padrão: gerado a partir do horário)")
	dataDir := flag.String("data-dir", train.DefaultDataDir, "diretório onde pesos e métricas são gravados")
	seed := flag.Int64("seed", 0, "semente do gerador aleatório (padrão: horário atual)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := train.Config{
		Episodes:           *episodes,
		LearningRate:       *learningRate,
		CheckpointInterval: *checkpointInterval,
		LogInterval:        *logInterval,
		RunID:              *runID,
		DataDir:            *dataDir,
		Seed:               *seed,
	}
	if err := cfg.Validate(); err != nil {
		logger.Error("configuração inválida", "error", err)
		flag.Usage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := train.Run(ctx, cfg, agent.NewDefault(), logger)
	if err != nil {
		logger.Error("treino abortado", "error", err)
		os.Exit(1)
	}
	if !result.Completed {
		os.Exit(130)
	}
}
