package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriter_AppendsOneLinePerRecord(t *testing.T) {
	path := RunFile(t.TempDir(), "run-a")

	writer, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	want := []Record{
		{Episode: 1, Score: 100, MaxTile: 64, Moves: 80},
		{Episode: 2, Score: 220, MaxTile: 128, Moves: 130},
		{Episode: 3, Score: 540, MaxTile: 256, Won: true, Moves: 210},
	}
	for _, record := range want {
		if err := writer.Append(record); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("esperadas %d linhas, obtidas %d", len(want), len(lines))
	}
	for i, line := range lines {
		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("linha %d não é JSON válido: %v", i, err)
		}
		if record != want[i] {
			t.Errorf("linha %d: %+v, esperado %+v", i, record, want[i])
		}
	}
}

func TestWriter_SyncsAfterEachAppend(t *testing.T) {
	path := RunFile(t.TempDir(), "run-a")

	writer, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { writer.Close() })

	if err := writer.Append(Record{Episode: 1, Score: 100, MaxTile: 64, Moves: 80}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	records, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll antes do Close: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("o registro deveria estar legível antes do Close, obtidos %d", len(records))
	}
}

func TestWriter_AppendsToExistingFile(t *testing.T) {
	path := RunFile(t.TempDir(), "run-a")

	for i := 1; i <= 2; i++ {
		writer, err := NewWriter(path)
		if err != nil {
			t.Fatalf("NewWriter: %v", err)
		}
		if err := writer.Append(Record{Episode: i}); err != nil {
			t.Fatalf("Append: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	records, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(records) != 2 || records[0].Episode != 1 || records[1].Episode != 2 {
		t.Errorf("esperado append preservando a ordem, obtido %+v", records)
	}
}

func TestNewWriter_CreatesRunDirectory(t *testing.T) {
	path := RunFile(t.TempDir(), "run-nova")

	writer, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer writer.Close()

	if writer.Path() != path {
		t.Errorf("Path %q, esperado %q", writer.Path(), path)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("diretório do run não foi criado: %v", err)
	}
}

func TestNewWriter_FailsWhenPathIsNotWritable(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := NewWriter(filepath.Join(blocked, DirName, FileName)); err == nil {
		t.Error("esperado erro ao abrir um caminho não gravável")
	}
}
