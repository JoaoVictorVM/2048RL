package train

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/metrics"
)

func TestGenerateRunID_MatchesExpectedFormat(t *testing.T) {
	at := time.Date(2026, 8, 16, 15, 30, 0, 0, time.UTC)

	if got, want := GenerateRunID(at), "run-20260816-153000"; got != want {
		t.Errorf("esperado %q, obtido %q", want, got)
	}
}

func TestGenerateRunID_UsesUTC(t *testing.T) {
	zone := time.FixedZone("UTC-3", -3*60*60)
	at := time.Date(2026, 8, 16, 12, 30, 0, 0, zone)

	if got, want := GenerateRunID(at), "run-20260816-153000"; got != want {
		t.Errorf("esperado %q em UTC, obtido %q", want, got)
	}
}

func TestResolveRunID_AutoSuffixesOnCollision(t *testing.T) {
	dataDir := t.TempDir()
	at := time.Date(2026, 8, 16, 15, 30, 0, 0, time.UTC)
	base := GenerateRunID(at)

	if err := os.MkdirAll(WeightsDir(dataDir, base), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	runID, err := ResolveRunID(dataDir, "", at)
	if err != nil {
		t.Fatalf("ResolveRunID: %v", err)
	}
	if want := base + "-2"; runID != want {
		t.Fatalf("esperado %q, obtido %q", want, runID)
	}

	if err := os.MkdirAll(filepath.Join(dataDir, metrics.DirName, runID), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	next, err := ResolveRunID(dataDir, "", at)
	if err != nil {
		t.Fatalf("ResolveRunID: %v", err)
	}
	if want := base + "-3"; next != want {
		t.Errorf("esperado %q, obtido %q", want, next)
	}
}

func TestResolveRunID_NoCollisionKeepsGeneratedID(t *testing.T) {
	at := time.Date(2026, 8, 16, 15, 30, 0, 0, time.UTC)

	runID, err := ResolveRunID(t.TempDir(), "", at)
	if err != nil {
		t.Fatalf("ResolveRunID: %v", err)
	}
	if runID != GenerateRunID(at) {
		t.Errorf("esperado o ID gerado sem sufixo, obtido %q", runID)
	}
}

func TestResolveRunID_ExplicitCollisionErrors(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(WeightsDir(dataDir, "meu-run"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	runID, err := ResolveRunID(dataDir, "meu-run", time.Now())
	if err == nil {
		t.Fatalf("esperado erro para run id explícito já existente, obtido %q", runID)
	}
	if !strings.Contains(err.Error(), "meu-run") {
		t.Errorf("o erro deveria citar o run id, obtido: %v", err)
	}
}

func TestResolveRunID_ExplicitIDIsKeptWhenFree(t *testing.T) {
	runID, err := ResolveRunID(t.TempDir(), "meu-run", time.Now())
	if err != nil {
		t.Fatalf("ResolveRunID: %v", err)
	}
	if runID != "meu-run" {
		t.Errorf("esperado meu-run, obtido %q", runID)
	}
}

func TestCheckpointPath_MatchesWebNamingConvention(t *testing.T) {
	got := CheckpointPath("/dados", "run-a", 1000)
	want := filepath.Join("/dados", "weights", "run-a", "weights_ep1000.bin")

	if got != want {
		t.Errorf("esperado %q, obtido %q", want, got)
	}
}
