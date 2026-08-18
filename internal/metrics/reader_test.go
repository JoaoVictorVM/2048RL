package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeRunFile(t *testing.T, dataDir, runID, content string, modTime time.Time) string {
	t.Helper()

	path := RunFile(dataDir, runID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return path
}

func TestReadAll_SkipsMalformedLines(t *testing.T) {
	content := "{\"episode\":1,\"score\":100,\"max_tile\":128,\"won\":false,\"moves\":90}\n" +
		"linha corrompida\n" +
		"\n" +
		"{\"episode\":2,\"score\":300,\"max_tile\":256,\"won\":true,\"moves\":180}\n"
	path := writeRunFile(t, t.TempDir(), "run-a", content, time.Now())

	records, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("esperados 2 registros válidos, obtidos %d", len(records))
	}
	if records[0].Episode != 1 || records[1].Episode != 2 {
		t.Errorf("ordem ou conteúdo inesperado: %+v", records)
	}
	if !records[1].Won || records[1].MaxTile != 256 {
		t.Errorf("registro 2 decodificado errado: %+v", records[1])
	}
}

func TestReadAll_MissingFileReturnsError(t *testing.T) {
	if _, err := ReadAll(filepath.Join(t.TempDir(), "ausente.jsonl")); err == nil {
		t.Error("esperado erro para arquivo inexistente")
	}
}

func TestMostRecentRunFile_PicksNewestByModTime(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now()

	writeRunFile(t, dataDir, "run-a", "{\"episode\":1}\n", now.Add(-3*time.Hour))
	writeRunFile(t, dataDir, "run-c", "{\"episode\":1}\n", now.Add(-1*time.Hour))
	writeRunFile(t, dataDir, "run-b", "{\"episode\":1}\n", now.Add(-2*time.Hour))

	runID, path, err := MostRecentRunFile(dataDir)
	if err != nil {
		t.Fatalf("MostRecentRunFile: %v", err)
	}
	if runID != "run-c" {
		t.Errorf("esperado run-c, obtido %q", runID)
	}
	if path != RunFile(dataDir, "run-c") {
		t.Errorf("caminho inesperado: %q", path)
	}
}

func TestMostRecentRunFile_NoRunsReturnsEmpty(t *testing.T) {
	runID, path, err := MostRecentRunFile(t.TempDir())
	if err != nil {
		t.Fatalf("MostRecentRunFile: %v", err)
	}
	if runID != "" || path != "" {
		t.Errorf("esperado retorno vazio, obtido %q / %q", runID, path)
	}
}

func TestMostRecentRunFile_MissingDataDirReturnsEmpty(t *testing.T) {
	runID, path, err := MostRecentRunFile(filepath.Join(t.TempDir(), "ausente"))
	if err != nil {
		t.Fatalf("MostRecentRunFile: %v", err)
	}
	if runID != "" || path != "" {
		t.Errorf("esperado retorno vazio, obtido %q / %q", runID, path)
	}
}

func TestMostRecentRunFile_IgnoresRunsWithoutMetricsFile(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, DirName, "run-sem-arquivo"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeRunFile(t, dataDir, "run-com-arquivo", "{\"episode\":1}\n", time.Now().Add(-time.Hour))

	runID, _, err := MostRecentRunFile(dataDir)
	if err != nil {
		t.Fatalf("MostRecentRunFile: %v", err)
	}
	if runID != "run-com-arquivo" {
		t.Errorf("esperado run-com-arquivo, obtido %q", runID)
	}
}
