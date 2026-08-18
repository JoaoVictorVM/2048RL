package metrics

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxLineBytes = 1 << 20

// Linhas malformadas são ignoradas: um treino interrompido no meio de uma
// escrita não deve inutilizar o histórico inteiro.
func ReadAll(path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []Record
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func MostRecentRunFile(dataDir string) (string, string, error) {
	entries, err := os.ReadDir(filepath.Join(dataDir, DirName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", nil
		}
		return "", "", err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var runID, path string
	var newest time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := RunFile(dataDir, entry.Name())
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if path == "" || info.ModTime().After(newest) {
			runID, path, newest = entry.Name(), candidate, info.ModTime()
		}
	}
	return runID, path, nil
}
