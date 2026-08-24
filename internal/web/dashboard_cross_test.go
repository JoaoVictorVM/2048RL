package web_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JoaoVictorVM/2048RL/internal/agent"
	"github.com/JoaoVictorVM/2048RL/internal/metrics"
	"github.com/JoaoVictorVM/2048RL/internal/train"
	"github.com/JoaoVictorVM/2048RL/internal/web"
)

// As métricas aqui saem de um treino real (F03) e são agregadas pelo endpoint
// do dashboard (F06), fechando o contrato entre as duas features.
func TestRunMetrics_SummarizesRealTrainingRunData(t *testing.T) {
	dataDir := t.TempDir()

	network, err := agent.New(crossAgentConfig())
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}

	const episodes = 6
	result, err := train.Run(context.Background(), train.Config{
		Episodes:           episodes,
		LearningRate:       0.1,
		CheckpointInterval: episodes,
		LogInterval:        episodes,
		RunID:              "run-treinado",
		DataDir:            dataDir,
		Seed:               2026,
	}, network, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("train.Run: %v", err)
	}

	srv := web.NewServer(web.Config{
		DataDir:   dataDir,
		StaticDir: t.TempDir(),
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/runs/" + result.RunID + "/metrics?window=3")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", resp.StatusCode)
	}

	var payload struct {
		RunID        string `json:"run_id"`
		WindowSize   int    `json:"window_size"`
		EpisodeCount int    `json:"episode_count"`
		Points       []struct {
			WindowStart int     `json:"window_start"`
			WindowEnd   int     `json:"window_end"`
			AvgScore    float64 `json:"avg_score"`
			AvgMaxTile  float64 `json:"avg_max_tile"`
			WinRate     float64 `json:"win_rate"`
		} `json:"points"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if payload.RunID != result.RunID {
		t.Errorf("run_id %q, esperado %q", payload.RunID, result.RunID)
	}
	if payload.EpisodeCount != episodes {
		t.Fatalf("episode_count %d, esperado %d", payload.EpisodeCount, episodes)
	}
	if len(payload.Points) != 2 {
		t.Fatalf("esperadas 2 janelas de 3 episódios, obtidas %d", len(payload.Points))
	}

	records, err := metrics.ReadAll(metrics.RunFile(dataDir, result.RunID))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	bounds := [][2]int{{0, 3}, {3, 6}}
	for i, point := range payload.Points {
		want := metrics.Summarize(records[bounds[i][0]:bounds[i][1]])
		if math.Abs(point.AvgScore-want.AvgScore) > 1e-9 {
			t.Errorf("janela %d: avg_score %v, esperado %v", i, point.AvgScore, want.AvgScore)
		}
		if math.Abs(point.AvgMaxTile-want.AvgMaxTile) > 1e-9 {
			t.Errorf("janela %d: avg_max_tile %v, esperado %v", i, point.AvgMaxTile, want.AvgMaxTile)
		}
		if math.Abs(point.WinRate-want.WinRate) > 1e-9 {
			t.Errorf("janela %d: win_rate %v, esperado %v", i, point.WinRate, want.WinRate)
		}
		if point.AvgScore <= 0 {
			t.Errorf("janela %d deveria ter score positivo vindo do treino real: %+v", i, point)
		}
	}

	if payload.Points[0].WindowStart != 1 || payload.Points[1].WindowEnd != episodes {
		t.Errorf("as janelas deveriam cobrir todo o histórico: %+v", payload.Points)
	}
}
