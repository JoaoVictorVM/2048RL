package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestServer(t *testing.T, cfg Config) *httptest.Server {
	t.Helper()
	if cfg.Logger == nil {
		cfg.Logger = discardLogger()
	}
	ts := httptest.NewServer(NewServer(cfg).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func getRuns(t *testing.T, baseURL string) RunsResponse {
	t.Helper()
	resp, err := http.Get(baseURL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected a JSON content type, got %q", ct)
	}

	var payload RunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return payload
}

func TestRunsEndpoint_ReturnsCurrentDirState(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-20260816-153000", "weights_ep1000.bin")
	writeCheckpoint(t, dataDir, "run-20260816-153000", "weights_ep2000.bin")
	writeMetrics(t, dataDir, "run-20260816-153000")

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})
	payload := getRuns(t, ts.URL)

	if len(payload.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(payload.Runs))
	}
	run := payload.Runs[0]
	if run.RunID != "run-20260816-153000" {
		t.Errorf("unexpected run_id %q", run.RunID)
	}
	if len(run.Checkpoints) != 2 {
		t.Fatalf("expected 2 checkpoints, got %d", len(run.Checkpoints))
	}
	if run.Checkpoints[0].Filename != "weights_ep1000.bin" || run.Checkpoints[0].Episode != 1000 {
		t.Errorf("unexpected checkpoint: %+v", run.Checkpoints[0])
	}
	if _, err := time.Parse(time.RFC3339, run.Checkpoints[0].ModifiedAt); err != nil {
		t.Errorf("modified_at is not RFC3339: %v", err)
	}
	if !run.HasMetrics || run.MetricsModifiedAt == nil {
		t.Errorf("expected metrics to be detected: %+v", run)
	}
}

func TestRunsEndpoint_DataDirMissingReturns500(t *testing.T) {
	ts := newTestServer(t, Config{
		DataDir:   filepath.Join(t.TempDir(), "absent"),
		StaticDir: t.TempDir(),
	})

	resp, err := http.Get(ts.URL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	var payload errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload.Error.Code != errCodeDataDir {
		t.Errorf("expected error code %s, got %q", errCodeDataDir, payload.Error.Code)
	}
}

func TestRunsEndpoint_ReflectsNewRunWithoutRestart(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-old", "weights_ep100.bin")

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})

	if runs := getRuns(t, ts.URL).Runs; len(runs) != 1 {
		t.Fatalf("expected 1 run before the new run is added, got %d", len(runs))
	}

	writeCheckpoint(t, dataDir, "run-new", "weights_ep100.bin")

	runs := getRuns(t, ts.URL).Runs
	if len(runs) != 2 {
		t.Fatalf("expected the new run to appear without a restart, got %d runs", len(runs))
	}
	findRun(t, runs, "run-new")
}

func TestStaticAssets_ServedCorrectly(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "app.js"), "console.log('rl2048');")

	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: staticDir})

	resp, err := http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatalf("GET /app.js: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "console.log('rl2048');" {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestUnknownRoute_Returns404(t *testing.T) {
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir()})

	resp, err := http.Get(ts.URL + "/does-not-exist")
	if err != nil {
		t.Fatalf("GET /does-not-exist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestServer_MissingStaticDirDoesNotBlockStartup(t *testing.T) {
	ts := newTestServer(t, Config{
		DataDir:   t.TempDir(),
		StaticDir: filepath.Join(t.TempDir(), "absent"),
	})

	resp, err := http.Get(ts.URL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected the API to work without a static dir, got %d", resp.StatusCode)
	}
}

func TestServer_StartsAndLogsListenAddress(t *testing.T) {
	var logs bytes.Buffer
	srv := NewServer(Config{
		DataDir:   t.TempDir(),
		StaticDir: t.TempDir(),
		Logger:    slog.New(slog.NewTextHandler(&logs, nil)),
	})

	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	addr := srv.Addr()
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	var err error
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://" + addr + "/api/runs")
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server did not answer on %s: %v", addr, err)
	}
	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Start returned an error after a graceful shutdown: %v", err)
	}

	if !strings.Contains(logs.String(), "server listening") || !strings.Contains(logs.String(), addr) {
		t.Errorf("expected the listen address to be logged, got: %s", logs.String())
	}
}

func TestRunsEndpointContract_StableShapeForLiveView(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin")

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})

	resp, err := http.Get(ts.URL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Runs []map[string]any `json:"runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(payload.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(payload.Runs))
	}

	run := payload.Runs[0]
	for _, key := range []string{"run_id", "checkpoints"} {
		if _, ok := run[key]; !ok {
			t.Errorf("missing field %q in the run object", key)
		}
	}

	checkpoints, ok := run["checkpoints"].([]any)
	if !ok || len(checkpoints) != 1 {
		t.Fatalf("unexpected checkpoints field: %v", run["checkpoints"])
	}
	checkpoint, ok := checkpoints[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected checkpoint entry: %v", checkpoints[0])
	}
	for _, key := range []string{"filename", "episode", "modified_at"} {
		if _, ok := checkpoint[key]; !ok {
			t.Errorf("missing field %q in the checkpoint object", key)
		}
	}
}

func TestRunsEndpointContract_StableShapeForDashboard(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin")
	writeMetrics(t, dataDir, "run-a")
	writeCheckpoint(t, dataDir, "run-b", "weights_ep1000.bin")

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})

	resp, err := http.Get(ts.URL + "/api/runs")
	if err != nil {
		t.Fatalf("GET /api/runs: %v", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Runs []map[string]any `json:"runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(payload.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(payload.Runs))
	}

	for _, run := range payload.Runs {
		for _, key := range []string{"run_id", "has_metrics", "metrics_modified_at"} {
			if _, ok := run[key]; !ok {
				t.Errorf("missing field %q in the run object", key)
			}
		}
		if run["has_metrics"] == false && run["metrics_modified_at"] != nil {
			t.Errorf("expected metrics_modified_at to be null when has_metrics is false: %v", run)
		}
	}
}
