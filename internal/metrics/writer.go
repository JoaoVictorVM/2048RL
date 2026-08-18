package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Writer struct {
	mu   sync.Mutex
	file *os.File
}

func RunFile(dataDir, runID string) string {
	return filepath.Join(dataDir, DirName, runID, FileName)
}

func NewWriter(path string) (*Writer, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("metrics: não foi possível criar o diretório %s: %w", dir, err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("metrics: não foi possível abrir %s para escrita: %w", path, err)
	}
	return &Writer{file: file}, nil
}

// Cada registro é sincronizado na hora para que o dashboard consiga acompanhar
// um treino ainda em andamento.
func (w *Writer) Append(record Record) error {
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("metrics: não foi possível codificar o episódio %d: %w", record.Episode, err)
	}
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Write(line); err != nil {
		return fmt.Errorf("metrics: falha ao gravar o episódio %d: %w", record.Episode, err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("metrics: falha ao sincronizar o episódio %d: %w", record.Episode, err)
	}
	return nil
}

func (w *Writer) Path() string {
	return w.file.Name()
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
