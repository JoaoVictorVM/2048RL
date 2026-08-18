package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JoaoVictorVM/2048RL/internal/metrics"
)

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeCheckpoint(t *testing.T, dataDir, runID, filename string) {
	t.Helper()
	writeFile(t, filepath.Join(dataDir, WeightsDirName, runID, filename), "weights")
}

func writeMetrics(t *testing.T, dataDir, runID string) {
	t.Helper()
	writeFile(t, metrics.RunFile(dataDir, runID), "{}\n")
}

func findRun(t *testing.T, runs []Run, runID string) Run {
	t.Helper()
	for _, run := range runs {
		if run.RunID == runID {
			return run
		}
	}
	t.Fatalf("run %q not found in %v", runID, runs)
	return Run{}
}

func TestScanRuns_ListsCheckpointsPerRun(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin")
	writeCheckpoint(t, dataDir, "run-a", "weights_ep2000.bin")
	writeCheckpoint(t, dataDir, "run-b", "weights_ep500.bin")

	runs, err := ScanRuns(dataDir)
	if err != nil {
		t.Fatalf("ScanRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d: %v", len(runs), runs)
	}

	runA := findRun(t, runs, "run-a")
	if len(runA.Checkpoints) != 2 {
		t.Fatalf("expected 2 checkpoints for run-a, got %d", len(runA.Checkpoints))
	}
	if runA.Checkpoints[0].Episode != 1000 || runA.Checkpoints[1].Episode != 2000 {
		t.Errorf("checkpoints not parsed/sorted by episode: %v", runA.Checkpoints)
	}
	if runA.Checkpoints[0].Filename != "weights_ep1000.bin" {
		t.Errorf("unexpected filename: %q", runA.Checkpoints[0].Filename)
	}
	if runA.Checkpoints[0].ModifiedAt == "" {
		t.Error("expected modified_at to be populated")
	}

	runB := findRun(t, runs, "run-b")
	if len(runB.Checkpoints) != 1 || runB.Checkpoints[0].Episode != 500 {
		t.Errorf("unexpected checkpoints for run-b: %v", runB.Checkpoints)
	}
}

func TestScanRuns_EmptyDataDir(t *testing.T) {
	runs, err := ScanRuns(t.TempDir())
	if err != nil {
		t.Fatalf("ScanRuns: %v", err)
	}
	if runs == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(runs) != 0 {
		t.Fatalf("expected no runs, got %v", runs)
	}
}

func TestEnsureDataDir_CreatesRunDirectoryTree(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	if err := EnsureDataDir(dataDir); err != nil {
		t.Fatalf("EnsureDataDir: %v", err)
	}

	for _, sub := range []string{WeightsDirName, metrics.DirName} {
		info, err := os.Stat(filepath.Join(dataDir, sub))
		if err != nil || !info.IsDir() {
			t.Errorf("expected %s/%s to exist: %v", dataDir, sub, err)
		}
	}

	runs, err := ScanRuns(dataDir)
	if err != nil {
		t.Fatalf("ScanRuns after EnsureDataDir: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected no runs, got %v", runs)
	}
}

func TestScanRuns_MissingDataDir(t *testing.T) {
	if _, err := ScanRuns(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("expected an error for a missing data directory")
	}
}

func TestScanRuns_IgnoresMalformedCheckpointFilenames(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin")
	writeCheckpoint(t, dataDir, "run-a", "notes.txt")
	writeCheckpoint(t, dataDir, "run-a", "weights_epXYZ.bin")
	writeCheckpoint(t, dataDir, "run-a", "weights_ep1000.bin.tmp")

	runs, err := ScanRuns(dataDir)
	if err != nil {
		t.Fatalf("ScanRuns: %v", err)
	}

	runA := findRun(t, runs, "run-a")
	if len(runA.Checkpoints) != 1 {
		t.Fatalf("expected malformed filenames to be skipped, got %v", runA.Checkpoints)
	}
}

func TestScanRuns_RunWithoutCheckpointsIsStillListed(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, WeightsDirName, "run-empty"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	runs, err := ScanRuns(dataDir)
	if err != nil {
		t.Fatalf("ScanRuns: %v", err)
	}

	run := findRun(t, runs, "run-empty")
	if run.Checkpoints == nil {
		t.Fatal("expected an empty checkpoint slice, got nil")
	}
	if len(run.Checkpoints) != 0 {
		t.Fatalf("expected no checkpoints, got %v", run.Checkpoints)
	}
}

func TestScanRuns_DetectsMetricsFilePresence(t *testing.T) {
	dataDir := t.TempDir()
	writeCheckpoint(t, dataDir, "run-with", "weights_ep100.bin")
	writeCheckpoint(t, dataDir, "run-without", "weights_ep100.bin")
	writeMetrics(t, dataDir, "run-with")

	runs, err := ScanRuns(dataDir)
	if err != nil {
		t.Fatalf("ScanRuns: %v", err)
	}

	withMetrics := findRun(t, runs, "run-with")
	if !withMetrics.HasMetrics {
		t.Error("expected has_metrics to be true")
	}
	if withMetrics.MetricsModifiedAt == nil || *withMetrics.MetricsModifiedAt == "" {
		t.Error("expected metrics_modified_at to be populated")
	}

	withoutMetrics := findRun(t, runs, "run-without")
	if withoutMetrics.HasMetrics {
		t.Error("expected has_metrics to be false")
	}
	if withoutMetrics.MetricsModifiedAt != nil {
		t.Errorf("expected metrics_modified_at to be nil, got %v", *withoutMetrics.MetricsModifiedAt)
	}
}
