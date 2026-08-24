package train

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/agent"
	"github.com/JoaoVictorVM/2048RL/internal/game"
	"github.com/JoaoVictorVM/2048RL/internal/metrics"
)

type Result struct {
	RunID     string
	Episodes  int
	Completed bool
	Overall   metrics.Summary
}

func Run(ctx context.Context, cfg Config, network *agent.Network, logger *slog.Logger) (Result, error) {
	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}
	if network == nil {
		return Result{}, fmt.Errorf("train: nenhuma rede foi fornecida")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	runID, err := ResolveRunID(cfg.DataDir, cfg.RunID, time.Now())
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(WeightsDir(cfg.DataDir, runID), 0o755); err != nil {
		return Result{}, fmt.Errorf("train: diretório de checkpoints não é gravável: %w", err)
	}

	writer, err := metrics.NewWriter(metrics.RunFile(cfg.DataDir, runID))
	if err != nil {
		return Result{}, fmt.Errorf("train: diretório de métricas não é gravável: %w", err)
	}
	defer writer.Close()

	seed := cfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	logger.Info("training started",
		"run_id", runID,
		"episodes", cfg.Episodes,
		"learning_rate", cfg.LearningRate,
		"checkpoint_interval", cfg.CheckpointInterval,
		"data_dir", cfg.DataDir,
		"seed", seed)

	result := Result{RunID: runID}
	all := make([]metrics.Record, 0, cfg.Episodes)
	window := make([]metrics.Record, 0, cfg.LogInterval)

	for episode := 1; episode <= cfg.Episodes; episode++ {
		if ctx.Err() != nil {
			logger.Info("training interrupted", "run_id", runID, "completed_episodes", episode-1)
			break
		}

		record := playEpisode(episode, network, cfg.LearningRate, rng)
		all = append(all, record)
		window = append(window, record)
		result.Episodes = episode

		if err := writer.Append(record); err != nil {
			logger.Error("failed to record episode metrics", "run_id", runID, "episode", episode, "error", err)
		}

		if episode%cfg.LogInterval == 0 {
			summary := metrics.Summarize(window)
			logger.Info("progress",
				"run_id", runID,
				"episode", episode,
				"avg_score", summary.AvgScore,
				"avg_max_tile", summary.AvgMaxTile,
				"win_rate", summary.WinRate)
			window = window[:0]
		}

		if episode%cfg.CheckpointInterval == 0 {
			saveCheckpoint(cfg.DataDir, runID, episode, network, logger)
		}
	}

	if result.Episodes > 0 {
		saveCheckpoint(cfg.DataDir, runID, result.Episodes, network, logger)
	}

	result.Completed = result.Episodes == cfg.Episodes
	result.Overall = metrics.Summarize(all)

	logger.Info("training finished",
		"run_id", runID,
		"episodes", result.Episodes,
		"completed", result.Completed,
		"avg_score", result.Overall.AvgScore,
		"avg_max_tile", result.Overall.AvgMaxTile,
		"win_rate", result.Overall.WinRate)

	return result, nil
}

// TD-afterstate com defasagem de um passo: o alvo do afterstate anterior é o
// valor do melhor movimento do estado atual (recompensa + valor do afterstate).
func playEpisode(episode int, network *agent.Network, learningRate float64, rng *rand.Rand) metrics.Record {
	g := game.NewGame(game.WithRNG(rng))

	var previous game.Board
	hasPrevious := false
	moves := 0

	for {
		evals := network.EvaluateMoves(g)
		if len(evals) == 0 {
			break
		}

		best := evals[0]
		for _, eval := range evals[1:] {
			if eval.Value > best.Value {
				best = eval
			}
		}

		if hasPrevious {
			network.Update(previous, learningRate*(best.Value-network.Evaluate(previous)))
		}

		g.ApplyMove(best.Direction)
		previous = best.Afterstate
		hasPrevious = true
		moves++
	}

	if hasPrevious {
		network.Update(previous, learningRate*(0-network.Evaluate(previous)))
	}

	return metrics.Record{
		Episode: episode,
		Score:   g.Score(),
		MaxTile: g.MaxTile(),
		Won:     g.IsWon(),
		Moves:   moves,
	}
}

// Uma falha ao salvar um checkpoint isolado não derruba o treino: o próximo
// intervalo tenta de novo e o histórico de métricas segue intacto.
func saveCheckpoint(dataDir, runID string, episode int, network *agent.Network, logger *slog.Logger) {
	path := CheckpointPath(dataDir, runID, episode)
	if err := network.Save(path); err != nil {
		logger.Error("failed to save checkpoint", "run_id", runID, "episode", episode, "path", path, "error", err)
		return
	}
	logger.Info("checkpoint saved", "run_id", runID, "episode", episode, "path", path)
}
