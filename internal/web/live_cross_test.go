package web_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/JoaoVictorVM/2048RL/internal/agent"
	"github.com/JoaoVictorVM/2048RL/internal/train"
	"github.com/JoaoVictorVM/2048RL/internal/web"
)

func crossAgentConfig() agent.Config {
	return agent.Config{
		MaxExponent: 5,
		Tuples: []agent.Tuple{
			{agent.Cell(0, 0), agent.Cell(0, 1), agent.Cell(1, 0), agent.Cell(1, 1)},
			{agent.Cell(0, 1), agent.Cell(0, 2), agent.Cell(1, 1), agent.Cell(1, 2)},
		},
	}
}

// O checkpoint aqui é gerado pelo treino de verdade (F03) e consumido pelo
// stream ao vivo (F05), fechando o contrato entre as duas features.
func TestLiveWS_LoadsRealCheckpointFromTraining(t *testing.T) {
	dataDir := t.TempDir()

	network, err := agent.New(crossAgentConfig())
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}

	result, err := train.Run(context.Background(), train.Config{
		Episodes:           2,
		LearningRate:       0.1,
		CheckpointInterval: 2,
		LogInterval:        2,
		RunID:              "run-treinado",
		DataDir:            dataDir,
		Seed:               2026,
	}, network, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("train.Run: %v", err)
	}

	srv := web.NewServer(web.Config{
		DataDir:     dataDir,
		StaticDir:   t.TempDir(),
		AgentConfig: crossAgentConfig(),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/ws/live", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	start := readCrossMessage(t, conn)
	if start["type"] != "episode_start" {
		t.Fatalf("esperado episode_start, obtido %v", start)
	}
	if start["run_id"] != result.RunID {
		t.Errorf("run_id %v, esperado %v", start["run_id"], result.RunID)
	}
	if start["checkpoint"] != "weights_ep2.bin" {
		t.Errorf("checkpoint %v, esperado weights_ep2.bin", start["checkpoint"])
	}

	move := readCrossMessage(t, conn)
	if move["type"] != "move" {
		t.Fatalf("esperado move, obtido %v", move)
	}
	if move["move_count"] != float64(1) {
		t.Errorf("move_count %v, esperado 1", move["move_count"])
	}
}

func readCrossMessage(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()

	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var message map[string]any
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatalf("unmarshal %s: %v", payload, err)
	}
	return message
}
