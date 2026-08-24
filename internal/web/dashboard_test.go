package web

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoaoVictorVM/2048RL/internal/metrics"
)

func writeEpisodes(t *testing.T, dataDir, runID string, count int) []metrics.Record {
	t.Helper()

	path := metrics.RunFile(dataDir, runID)
	writer, err := metrics.NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer writer.Close()

	records := make([]metrics.Record, 0, count)
	for episode := 1; episode <= count; episode++ {
		record := metrics.Record{
			Episode: episode,
			Score:   100 * episode,
			MaxTile: 1 << (2 + episode%6),
			Won:     episode%10 == 0,
			Moves:   50 + episode,
		}
		if err := writer.Append(record); err != nil {
			t.Fatalf("Append: %v", err)
		}
		records = append(records, record)
	}
	return records
}

func getRunMetrics(t *testing.T, baseURL, runID, query string) runMetricsResponse {
	t.Helper()

	url := fmt.Sprintf("%s/api/runs/%s/metrics", baseURL, runID)
	if query != "" {
		url += "?" + query
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", resp.StatusCode)
	}

	var payload runMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload
}

func assertClose(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: %v, esperado %v", label, got, want)
	}
}

func TestRunMetrics_AggregatesIntoDefaultWindows(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep100.bin")
	records := writeEpisodes(t, dataDir, "run-a", 250)

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})
	payload := getRunMetrics(t, ts.URL, "run-a", "")

	if payload.RunID != "run-a" {
		t.Errorf("run_id %q", payload.RunID)
	}
	if payload.WindowSize != DefaultMetricsWindow {
		t.Errorf("window_size %d, esperado %d", payload.WindowSize, DefaultMetricsWindow)
	}
	if payload.EpisodeCount != 250 {
		t.Errorf("episode_count %d, esperado 250", payload.EpisodeCount)
	}
	if len(payload.Points) != 3 {
		t.Fatalf("esperados 3 pontos, obtidos %d", len(payload.Points))
	}

	bounds := [][2]int{{1, 100}, {101, 200}, {201, 250}}
	for i, point := range payload.Points {
		if point.WindowStart != bounds[i][0] || point.WindowEnd != bounds[i][1] {
			t.Errorf("ponto %d: janela %d-%d, esperada %d-%d",
				i, point.WindowStart, point.WindowEnd, bounds[i][0], bounds[i][1])
		}

		want := metrics.Summarize(records[bounds[i][0]-1 : bounds[i][1]])
		assertClose(t, fmt.Sprintf("ponto %d avg_score", i), point.AvgScore, want.AvgScore)
		assertClose(t, fmt.Sprintf("ponto %d avg_max_tile", i), point.AvgMaxTile, want.AvgMaxTile)
		assertClose(t, fmt.Sprintf("ponto %d win_rate", i), point.WinRate, want.WinRate)
	}
}

func TestRunMetrics_WindowOverrideViaQueryParam(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep100.bin")
	writeEpisodes(t, dataDir, "run-a", 250)

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})
	payload := getRunMetrics(t, ts.URL, "run-a", "window=50")

	if payload.WindowSize != 50 {
		t.Errorf("window_size %d, esperado 50", payload.WindowSize)
	}
	if len(payload.Points) != 5 {
		t.Fatalf("esperados 5 pontos, obtidos %d", len(payload.Points))
	}
	if payload.Points[0].WindowStart != 1 || payload.Points[0].WindowEnd != 50 {
		t.Errorf("primeira janela inesperada: %+v", payload.Points[0])
	}
	if last := payload.Points[4]; last.WindowStart != 201 || last.WindowEnd != 250 {
		t.Errorf("última janela inesperada: %+v", last)
	}
}

func TestRunMetrics_InvalidWindowFallsBackToDefault(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep100.bin")
	writeEpisodes(t, dataDir, "run-a", 10)

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})

	for _, query := range []string{"window=0", "window=-5", "window=abc"} {
		payload := getRunMetrics(t, ts.URL, "run-a", query)
		if payload.WindowSize != DefaultMetricsWindow {
			t.Errorf("%s: window_size %d, esperado o padrão %d", query, payload.WindowSize, DefaultMetricsWindow)
		}
	}
}

func TestRunMetrics_NoMetricsYetReturnsEmptyPoints(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-sem-metricas", "weights_ep100.bin")

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})
	payload := getRunMetrics(t, ts.URL, "run-sem-metricas", "")

	if payload.EpisodeCount != 0 {
		t.Errorf("episode_count %d, esperado 0", payload.EpisodeCount)
	}
	if payload.Points == nil {
		t.Fatal("points deveria ser uma lista vazia, não nulo")
	}
	if len(payload.Points) != 0 {
		t.Errorf("esperada nenhuma janela, obtidas %d", len(payload.Points))
	}
}

func TestRunMetrics_UnknownRunReturns404(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep100.bin")

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})

	resp, err := http.Get(ts.URL + "/api/runs/run-inexistente/metrics")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("esperado 404, obtido %d", resp.StatusCode)
	}
	if code := decodeError(t, resp).Code; code != errCodeUnknownRun {
		t.Errorf("esperado código %s, obtido %s", errCodeUnknownRun, code)
	}
}

func TestRunMetrics_MissingDataDirReturns404(t *testing.T) {
	ts := newTestServer(t, Config{
		DataDir:   filepath.Join(t.TempDir(), "ausente"),
		StaticDir: t.TempDir(),
	})

	resp, err := http.Get(ts.URL + "/api/runs/run-a/metrics")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("esperado 404, obtido %d", resp.StatusCode)
	}
}

func TestRunMetrics_SkipsMalformedRecords(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep100.bin")

	path := metrics.RunFile(dataDir, "run-a")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "{\"episode\":1,\"score\":100,\"max_tile\":128,\"won\":false}\n" +
		"linha corrompida\n" +
		"{\"episode\":2,\"score\":300,\"max_tile\":256,\"won\":true}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})
	payload := getRunMetrics(t, ts.URL, "run-a", "window=10")

	if payload.EpisodeCount != 2 {
		t.Fatalf("episode_count %d, esperado 2", payload.EpisodeCount)
	}
	if len(payload.Points) != 1 {
		t.Fatalf("esperada 1 janela, obtidas %d", len(payload.Points))
	}
	assertClose(t, "avg_score", payload.Points[0].AvgScore, 200)
	assertClose(t, "win_rate", payload.Points[0].WinRate, 0.5)
}

func TestRunMetrics_RunValidationMatchesRunsEndpoint(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep100.bin")
	writeCheckpoint(t, dataDir, "run-b", "weights_ep200.bin")
	writeEpisodes(t, dataDir, "run-a", 5)

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})

	listed := getRuns(t, ts.URL)
	if len(listed.Runs) != 2 {
		t.Fatalf("esperados 2 runs listados, obtidos %d", len(listed.Runs))
	}

	for _, run := range listed.Runs {
		payload := getRunMetrics(t, ts.URL, run.RunID, "")
		if payload.RunID != run.RunID {
			t.Errorf("run %q respondeu com run_id %q", run.RunID, payload.RunID)
		}
		if run.HasMetrics && payload.EpisodeCount == 0 {
			t.Errorf("run %q tem métricas em /api/runs mas nenhum episódio no dashboard", run.RunID)
		}
		if !run.HasMetrics && payload.EpisodeCount != 0 {
			t.Errorf("run %q não tem métricas em /api/runs mas o dashboard achou %d episódios",
				run.RunID, payload.EpisodeCount)
		}
	}
}

func TestWindowPoints_HandlesEmptyAndPartialWindows(t *testing.T) {
	if points := windowPoints(nil, 10); len(points) != 0 {
		t.Errorf("esperada nenhuma janela para histórico vazio, obtidas %d", len(points))
	}

	records := []metrics.Record{
		{Episode: 7, Score: 100, MaxTile: 64},
		{Episode: 8, Score: 300, MaxTile: 128, Won: true},
	}
	points := windowPoints(records, 10)
	if len(points) != 1 {
		t.Fatalf("esperada 1 janela parcial, obtidas %d", len(points))
	}
	if points[0].WindowStart != 7 || points[0].WindowEnd != 8 {
		t.Errorf("a janela deveria usar os números reais dos episódios: %+v", points[0])
	}
	assertClose(t, "avg_score", points[0].AvgScore, 200)
	assertClose(t, "avg_max_tile", points[0].AvgMaxTile, 96)
	assertClose(t, "win_rate", points[0].WinRate, 0.5)
}

func TestRunMetrics_ReflectsNewEpisodesWithoutRestart(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep100.bin")
	writeEpisodes(t, dataDir, "run-a", 3)

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})

	if payload := getRunMetrics(t, ts.URL, "run-a", "window=10"); payload.EpisodeCount != 3 {
		t.Fatalf("episode_count inicial %d, esperado 3", payload.EpisodeCount)
	}

	writer, err := metrics.NewWriter(metrics.RunFile(dataDir, "run-a"))
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := writer.Append(metrics.Record{Episode: 4, Score: 400, MaxTile: 512, Won: true}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	payload := getRunMetrics(t, ts.URL, "run-a", "window=10")
	if payload.EpisodeCount != 4 {
		t.Errorf("episode_count após novo episódio %d, esperado 4", payload.EpisodeCount)
	}
	if payload.Points[0].WindowEnd != 4 {
		t.Errorf("a janela deveria incluir o episódio novo: %+v", payload.Points[0])
	}
}
