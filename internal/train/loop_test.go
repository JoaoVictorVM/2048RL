package train

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoaoVictorVM/2048RL/internal/agent"
	"github.com/JoaoVictorVM/2048RL/internal/game"
	"github.com/JoaoVictorVM/2048RL/internal/metrics"
)

func testAgentConfig() agent.Config {
	return agent.Config{
		MaxExponent: 5,
		Tuples: []agent.Tuple{
			{agent.Cell(0, 0), agent.Cell(0, 1), agent.Cell(1, 0), agent.Cell(1, 1)},
			{agent.Cell(0, 1), agent.Cell(0, 2), agent.Cell(1, 1), agent.Cell(1, 2)},
		},
	}
}

func newTestNetwork(t *testing.T) *agent.Network {
	t.Helper()

	network, err := agent.New(testAgentConfig())
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}
	return network
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testConfig(dataDir string, episodes int) Config {
	return Config{
		Episodes:           episodes,
		LearningRate:       0.1,
		CheckpointInterval: episodes,
		LogInterval:        episodes,
		RunID:              "run-teste",
		DataDir:            dataDir,
		Seed:               2026,
	}
}

func fixtureBoard() game.Board {
	return game.Board{
		{2, 4, 8, 0},
		{4, 2, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
}

func readRunMetrics(t *testing.T, dataDir, runID string) []metrics.Record {
	t.Helper()

	records, err := metrics.ReadAll(metrics.RunFile(dataDir, runID))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return records
}

func TestRun_ExecutesExactlyConfiguredEpisodes(t *testing.T) {
	dataDir := t.TempDir()

	result, err := Run(context.Background(), testConfig(dataDir, 5), newTestNetwork(t), discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Completed || result.Episodes != 5 {
		t.Fatalf("resultado inesperado: %+v", result)
	}

	records := readRunMetrics(t, dataDir, result.RunID)
	if len(records) != 5 {
		t.Fatalf("esperados 5 registros, obtidos %d", len(records))
	}
	for i, record := range records {
		if record.Episode != i+1 {
			t.Errorf("registro %d com episódio %d", i, record.Episode)
		}
		if record.Moves <= 0 || record.Score <= 0 {
			t.Errorf("episódio %d não parece ter sido jogado: %+v", record.Episode, record)
		}
		if record.MaxTile < 2 {
			t.Errorf("episódio %d com max tile inválido: %+v", record.Episode, record)
		}
	}
}

func TestRun_SavesCheckpointsAtIntervalAndAtEnd(t *testing.T) {
	dataDir := t.TempDir()
	cfg := testConfig(dataDir, 5)
	cfg.CheckpointInterval = 2

	result, err := Run(context.Background(), cfg, newTestNetwork(t), discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, episode := range []int{2, 4, 5} {
		if _, err := os.Stat(CheckpointPath(dataDir, result.RunID, episode)); err != nil {
			t.Errorf("checkpoint do episódio %d ausente: %v", episode, err)
		}
	}
	for _, episode := range []int{1, 3} {
		if _, err := os.Stat(CheckpointPath(dataDir, result.RunID, episode)); err == nil {
			t.Errorf("checkpoint do episódio %d não deveria existir", episode)
		}
	}
}

func TestRun_WeightsChangeAfterTraining(t *testing.T) {
	network := newTestNetwork(t)

	before := network.Evaluate(fixtureBoard())
	if _, err := Run(context.Background(), testConfig(t.TempDir(), 5), network, discardLogger()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if after := network.Evaluate(fixtureBoard()); after == before {
		t.Errorf("a rede deveria ter aprendido algo, valor seguiu em %v", after)
	}
}

func TestRun_ParallelRunsDoNotInterfere(t *testing.T) {
	dataDir := t.TempDir()

	first := testConfig(dataDir, 3)
	first.RunID = "run-a"
	second := testConfig(dataDir, 4)
	second.RunID = "run-b"

	if _, err := Run(context.Background(), first, newTestNetwork(t), discardLogger()); err != nil {
		t.Fatalf("Run run-a: %v", err)
	}
	if _, err := Run(context.Background(), second, newTestNetwork(t), discardLogger()); err != nil {
		t.Fatalf("Run run-b: %v", err)
	}

	if got := len(readRunMetrics(t, dataDir, "run-a")); got != 3 {
		t.Errorf("run-a deveria ter 3 registros, tem %d", got)
	}
	if got := len(readRunMetrics(t, dataDir, "run-b")); got != 4 {
		t.Errorf("run-b deveria ter 4 registros, tem %d", got)
	}
	if _, err := os.Stat(CheckpointPath(dataDir, "run-a", 3)); err != nil {
		t.Errorf("checkpoint de run-a ausente: %v", err)
	}
	if _, err := os.Stat(CheckpointPath(dataDir, "run-b", 4)); err != nil {
		t.Errorf("checkpoint de run-b ausente: %v", err)
	}
	if _, err := os.Stat(CheckpointPath(dataDir, "run-a", 4)); err == nil {
		t.Error("run-a não deveria conter o checkpoint de run-b")
	}
}

func TestRun_ExistingRunIDIsRejected(t *testing.T) {
	dataDir := t.TempDir()
	cfg := testConfig(dataDir, 2)

	if _, err := Run(context.Background(), cfg, newTestNetwork(t), discardLogger()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := Run(context.Background(), cfg, newTestNetwork(t), discardLogger()); err == nil {
		t.Error("esperado erro ao reutilizar um run id explícito")
	}
}

func TestRun_AbortsBeforeAnyEpisodeWhenDataDirNotWritable(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(blocked, []byte("nao e um diretorio"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Run(context.Background(), testConfig(blocked, 3), newTestNetwork(t), discardLogger()); err == nil {
		t.Fatal("esperado erro quando o diretório de dados não é gravável")
	}
	if _, err := os.Stat(metrics.RunFile(blocked, "run-teste")); err == nil {
		t.Error("nenhum arquivo de métricas deveria ter sido criado")
	}
}

func TestRun_StopsEarlyWhenContextIsCancelled(t *testing.T) {
	dataDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Run(ctx, testConfig(dataDir, 5), newTestNetwork(t), discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Completed || result.Episodes != 0 {
		t.Errorf("esperada parada antes do primeiro episódio: %+v", result)
	}
	if got := len(readRunMetrics(t, dataDir, result.RunID)); got != 0 {
		t.Errorf("esperado nenhum registro, obtidos %d", got)
	}
}

func TestRun_RejectsInvalidConfig(t *testing.T) {
	if _, err := Run(context.Background(), Config{}, newTestNetwork(t), discardLogger()); err == nil {
		t.Error("esperado erro para configuração inválida")
	}
}

func TestRun_RejectsMissingNetwork(t *testing.T) {
	if _, err := Run(context.Background(), testConfig(t.TempDir(), 1), nil, discardLogger()); err == nil {
		t.Error("esperado erro quando nenhuma rede é fornecida")
	}
}

func TestCheckpoint_LoadableByAgentPackageAfterTraining(t *testing.T) {
	dataDir := t.TempDir()
	network := newTestNetwork(t)

	result, err := Run(context.Background(), testConfig(dataDir, 3), network, discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	path := CheckpointPath(dataDir, result.RunID, result.Episodes)
	loaded, err := agent.LoadNetwork(testAgentConfig(), path)
	if err != nil {
		t.Fatalf("LoadNetwork: %v", err)
	}

	if got, want := loaded.Evaluate(fixtureBoard()), network.Evaluate(fixtureBoard()); got != want {
		t.Errorf("checkpoint carregado avalia %v, esperado %v", got, want)
	}
}

func TestMetrics_ReadableAfterTraining(t *testing.T) {
	dataDir := t.TempDir()

	result, err := Run(context.Background(), testConfig(dataDir, 3), newTestNetwork(t), discardLogger())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	runID, path, err := metrics.MostRecentRunFile(dataDir)
	if err != nil {
		t.Fatalf("MostRecentRunFile: %v", err)
	}
	if runID != result.RunID {
		t.Errorf("run id %q, esperado %q", runID, result.RunID)
	}

	records, err := metrics.ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("esperados 3 registros, obtidos %d", len(records))
	}
}

func TestRun_ContinuesAfterCheckpointWriteFailure(t *testing.T) {
	dataDir := t.TempDir()
	cfg := testConfig(dataDir, 3)
	cfg.CheckpointInterval = 1
	cfg.LogInterval = 1

	weightsDir := WeightsDir(dataDir, cfg.RunID)
	logger := slog.New(&blockingHandler{
		onProgress: func() {
			os.RemoveAll(weightsDir)
			os.WriteFile(weightsDir, []byte("nao e um diretorio"), 0o644)
		},
	})

	result, err := Run(context.Background(), cfg, newTestNetwork(t), logger)
	if err != nil {
		t.Fatalf("Run não deveria abortar por falha de checkpoint: %v", err)
	}
	if !result.Completed || result.Episodes != 3 {
		t.Fatalf("o treino deveria completar todos os episódios: %+v", result)
	}

	records := readRunMetrics(t, dataDir, cfg.RunID)
	if len(records) != 3 {
		t.Errorf("esperados 3 registros de métricas, obtidos %d", len(records))
	}
	if info, err := os.Stat(weightsDir); err != nil || info.IsDir() {
		t.Errorf("o diretório de pesos deveria seguir bloqueado durante o run: %v", err)
	}
}

// Substitui o diretório de pesos por um arquivo assim que o primeiro resumo é
// logado, simulando uma falha de disco no meio do treino.
type blockingHandler struct {
	onProgress func()
	fired      bool
}

func (h *blockingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *blockingHandler) Handle(_ context.Context, record slog.Record) error {
	if !h.fired && record.Message == "progress" {
		h.fired = true
		h.onProgress()
	}
	return nil
}

func (h *blockingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *blockingHandler) WithGroup(string) slog.Handler { return h }
