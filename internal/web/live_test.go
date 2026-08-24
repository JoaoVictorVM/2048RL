package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/JoaoVictorVM/2048RL/internal/agent"
)

func liveAgentConfig() agent.Config {
	return agent.Config{
		MaxExponent: 5,
		Tuples: []agent.Tuple{
			{agent.Cell(0, 0), agent.Cell(0, 1), agent.Cell(1, 0), agent.Cell(1, 1)},
			{agent.Cell(0, 1), agent.Cell(0, 2), agent.Cell(1, 1), agent.Cell(1, 2)},
		},
	}
}

func writeLiveCheckpoint(t *testing.T, dataDir, runID, filename string, modTime time.Time) string {
	t.Helper()

	network, err := agent.New(liveAgentConfig())
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}

	path := filepath.Join(dataDir, WeightsDirName, runID, filename)
	if err := network.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return path
}

func newLiveServer(t *testing.T, dataDir string) *httptest.Server {
	t.Helper()

	srv := NewServer(Config{
		DataDir:     dataDir,
		StaticDir:   t.TempDir(),
		AgentConfig: liveAgentConfig(),
		Logger:      discardLogger(),
	})
	srv.live = livePacing{moveDelay: time.Millisecond, restartDelay: 5 * time.Millisecond}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func dialLive(t *testing.T, baseURL, query string) *websocket.Conn {
	t.Helper()

	url := "ws" + strings.TrimPrefix(baseURL, "http") + "/ws/live"
	if query != "" {
		url += "?" + query
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func readLiveMessage(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

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

func readUntilType(t *testing.T, conn *websocket.Conn, wanted string, limit int) map[string]any {
	t.Helper()

	for i := 0; i < limit; i++ {
		message := readLiveMessage(t, conn)
		if message["type"] == wanted {
			return message
		}
	}
	t.Fatalf("mensagem %q não chegou em %d mensagens", wanted, limit)
	return nil
}

func TestLiveDelays_MatchPRDDefaults(t *testing.T) {
	if DefaultMoveDelay != 300*time.Millisecond {
		t.Errorf("DefaultMoveDelay %v, esperado 300ms", DefaultMoveDelay)
	}
	if DefaultEpisodeRestartDelay != 2*time.Second {
		t.Errorf("DefaultEpisodeRestartDelay %v, esperado 2s", DefaultEpisodeRestartDelay)
	}
}

func TestLiveWS_ExplicitCheckpointStartsEpisode(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", now.Add(-time.Hour))
	writeLiveCheckpoint(t, dataDir, "run-b", "weights_ep2000.bin", now)

	ts := newLiveServer(t, dataDir)
	conn := dialLive(t, ts.URL, "run_id=run-a&checkpoint=weights_ep1000.bin")

	message := readLiveMessage(t, conn)
	if message["type"] != "episode_start" {
		t.Fatalf("primeira mensagem deveria ser episode_start, obtido %v", message)
	}
	if message["run_id"] != "run-a" || message["checkpoint"] != "weights_ep1000.bin" {
		t.Errorf("seleção não ecoada: %v", message)
	}
	if message["score"] != float64(0) {
		t.Errorf("score inicial deveria ser 0: %v", message["score"])
	}

	board, ok := message["board"].([]any)
	if !ok || len(board) != 4 {
		t.Fatalf("board inesperado: %v", message["board"])
	}
	tiles := 0
	for _, row := range board {
		for _, cell := range row.([]any) {
			if cell.(float64) != 0 {
				tiles++
			}
		}
	}
	if tiles != 2 {
		t.Errorf("esperados 2 tiles iniciais, obtidos %d", tiles)
	}
}

func TestLiveWS_DefaultsToLatestCheckpointWhenUnspecified(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", now.Add(-2*time.Hour))
	writeLiveCheckpoint(t, dataDir, "run-b", "weights_ep500.bin", now)
	writeLiveCheckpoint(t, dataDir, "run-c", "weights_ep3000.bin", now.Add(-time.Hour))

	ts := newLiveServer(t, dataDir)
	conn := dialLive(t, ts.URL, "")

	message := readLiveMessage(t, conn)
	if message["run_id"] != "run-b" || message["checkpoint"] != "weights_ep500.bin" {
		t.Errorf("esperado o checkpoint mais recente (run-b), obtido %v", message)
	}
}

func TestLiveWS_DefaultsToLatestWithinRequestedRun(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", now.Add(-time.Hour))
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep2000.bin", now.Add(-30*time.Minute))
	writeLiveCheckpoint(t, dataDir, "run-b", "weights_ep100.bin", now)

	ts := newLiveServer(t, dataDir)
	conn := dialLive(t, ts.URL, "run_id=run-a")

	message := readLiveMessage(t, conn)
	if message["run_id"] != "run-a" || message["checkpoint"] != "weights_ep2000.bin" {
		t.Errorf("esperado o checkpoint mais recente de run-a, obtido %v", message)
	}
}

func TestLiveWS_MovesStreamInOrder(t *testing.T) {
	dataDir := t.TempDir()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", time.Now())

	ts := newLiveServer(t, dataDir)
	conn := dialLive(t, ts.URL, "run_id=run-a&checkpoint=weights_ep1000.bin")

	if message := readLiveMessage(t, conn); message["type"] != "episode_start" {
		t.Fatalf("esperado episode_start, obtido %v", message)
	}

	moves := 0
	lastScore := 0.0
	for {
		message := readLiveMessage(t, conn)
		if message["type"] == "episode_end" {
			if moves == 0 {
				t.Fatal("o episódio deveria ter ao menos uma jogada")
			}
			if message["move_count"] != float64(moves) {
				t.Errorf("move_count final %v, esperado %d", message["move_count"], moves)
			}
			if message["score"].(float64) < lastScore {
				t.Errorf("score final %v menor que o último visto %v", message["score"], lastScore)
			}
			if message["max_tile"].(float64) < 2 {
				t.Errorf("max_tile final inválido: %v", message["max_tile"])
			}
			return
		}
		if message["type"] != "move" {
			t.Fatalf("mensagem inesperada antes do fim do episódio: %v", message)
		}

		moves++
		if message["move_count"] != float64(moves) {
			t.Fatalf("move_count %v, esperado %d", message["move_count"], moves)
		}
		if score := message["score"].(float64); score < lastScore {
			t.Fatalf("score regrediu de %v para %v", lastScore, score)
		} else {
			lastScore = score
		}
		switch message["direction"] {
		case "up", "down", "left", "right":
		default:
			t.Fatalf("direção inválida: %v", message["direction"])
		}
		if moves > 5000 {
			t.Fatal("episódio não terminou dentro do limite do teste")
		}
	}
}

func TestLiveWS_NewEpisodeAutoStartsAfterGameOver(t *testing.T) {
	dataDir := t.TempDir()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", time.Now())

	ts := newLiveServer(t, dataDir)
	conn := dialLive(t, ts.URL, "run_id=run-a&checkpoint=weights_ep1000.bin")

	if message := readLiveMessage(t, conn); message["type"] != "episode_start" {
		t.Fatalf("esperado episode_start, obtido %v", message)
	}
	readUntilType(t, conn, "episode_end", 5000)

	second := readUntilType(t, conn, "episode_start", 5)
	if second["run_id"] != "run-a" || second["checkpoint"] != "weights_ep1000.bin" {
		t.Errorf("o novo episódio deveria usar o mesmo checkpoint: %v", second)
	}
	if second["score"] != float64(0) {
		t.Errorf("o novo episódio deveria começar zerado: %v", second["score"])
	}
}

func TestLiveWS_UnknownCheckpointSendsErrorAndCloses(t *testing.T) {
	dataDir := t.TempDir()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", time.Now())

	ts := newLiveServer(t, dataDir)
	conn := dialLive(t, ts.URL, "run_id=run-a&checkpoint=weights_ep9999.bin")

	message := readLiveMessage(t, conn)
	if message["type"] != "error" || message["code"] != errCodeCheckpointNotFound {
		t.Fatalf("esperado erro %s, obtido %v", errCodeCheckpointNotFound, message)
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Error("a conexão deveria ser encerrada após o erro")
	}
}

func TestLiveWS_UnknownRunSendsError(t *testing.T) {
	dataDir := t.TempDir()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", time.Now())

	ts := newLiveServer(t, dataDir)
	conn := dialLive(t, ts.URL, "run_id=run-inexistente")

	message := readLiveMessage(t, conn)
	if message["type"] != "error" || message["code"] != errCodeCheckpointNotFound {
		t.Fatalf("esperado erro %s, obtido %v", errCodeCheckpointNotFound, message)
	}
}

func TestLiveWS_NoCheckpointsAnywhereSendsError(t *testing.T) {
	ts := newLiveServer(t, t.TempDir())
	conn := dialLive(t, ts.URL, "")

	message := readLiveMessage(t, conn)
	if message["type"] != "error" || message["code"] != errCodeNoCheckpoints {
		t.Fatalf("esperado erro %s, obtido %v", errCodeNoCheckpoints, message)
	}
}

func TestLiveWS_MissingDataDirSendsNoCheckpointsError(t *testing.T) {
	ts := newLiveServer(t, filepath.Join(t.TempDir(), "ausente"))
	conn := dialLive(t, ts.URL, "")

	message := readLiveMessage(t, conn)
	if message["code"] != errCodeNoCheckpoints {
		t.Fatalf("esperado erro %s, obtido %v", errCodeNoCheckpoints, message)
	}
}

func TestLiveWS_CorruptedCheckpointSendsLoadError(t *testing.T) {
	dataDir := t.TempDir()
	path := writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", time.Now())

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(path, content[:len(content)/2], 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ts := newLiveServer(t, dataDir)
	conn := dialLive(t, ts.URL, "run_id=run-a&checkpoint=weights_ep1000.bin")

	message := readLiveMessage(t, conn)
	if message["type"] != "error" || message["code"] != errCodeCheckpointLoad {
		t.Fatalf("esperado erro %s, obtido %v", errCodeCheckpointLoad, message)
	}
}

func TestLiveWS_IndependentConnectionsUseTheirOwnCheckpoint(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", now.Add(-time.Hour))
	writeLiveCheckpoint(t, dataDir, "run-b", "weights_ep2000.bin", now)

	ts := newLiveServer(t, dataDir)

	first := dialLive(t, ts.URL, "run_id=run-a&checkpoint=weights_ep1000.bin")
	second := dialLive(t, ts.URL, "run_id=run-b&checkpoint=weights_ep2000.bin")

	firstStart := readLiveMessage(t, first)
	secondStart := readLiveMessage(t, second)

	if firstStart["checkpoint"] != "weights_ep1000.bin" {
		t.Errorf("primeira conexão: %v", firstStart)
	}
	if secondStart["checkpoint"] != "weights_ep2000.bin" {
		t.Errorf("segunda conexão: %v", secondStart)
	}
}

func TestLiveWS_RunResolutionMatchesRunsEndpoint(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now()
	writeLiveCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin", now.Add(-time.Hour))
	writeLiveCheckpoint(t, dataDir, "run-b", "weights_ep2000.bin", now)

	runs, err := ScanRuns(dataDir)
	if err != nil {
		t.Fatalf("ScanRuns: %v", err)
	}

	for _, run := range runs {
		for _, checkpoint := range run.Checkpoints {
			ref, liveErr := resolveCheckpoint(dataDir, run.RunID, checkpoint.Filename)
			if liveErr != nil {
				t.Fatalf("%s/%s deveria resolver: %v", run.RunID, checkpoint.Filename, liveErr)
			}
			if ref.RunID != run.RunID || ref.Filename != checkpoint.Filename {
				t.Errorf("resolução divergente: %+v", ref)
			}
			if _, err := os.Stat(ref.Path); err != nil {
				t.Errorf("caminho resolvido não existe: %v", err)
			}
		}
	}

	if _, liveErr := resolveCheckpoint(dataDir, "run-a", "weights_ep2000.bin"); liveErr == nil {
		t.Error("um checkpoint de outro run não deveria resolver dentro de run-a")
	}
}
