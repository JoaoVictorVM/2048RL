package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/metrics"
)

const (
	WeightsDirName = "weights"

	errCodeDataDir = "RUNS001"
)

func CheckpointFilename(episode int) string {
	return fmt.Sprintf("weights_ep%d.bin", episode)
}

var checkpointPattern = regexp.MustCompile(`^weights_ep(\d+)\.bin$`)

type Checkpoint struct {
	Filename   string `json:"filename"`
	Episode    int    `json:"episode"`
	ModifiedAt string `json:"modified_at"`
}

type Run struct {
	RunID             string       `json:"run_id"`
	Checkpoints       []Checkpoint `json:"checkpoints"`
	HasMetrics        bool         `json:"has_metrics"`
	MetricsModifiedAt *string      `json:"metrics_modified_at"`
}

type RunsResponse struct {
	Runs []Run `json:"runs"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func EnsureDataDir(dataDir string) error {
	for _, sub := range []string{WeightsDirName, metrics.DirName} {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func ScanRuns(dataDir string) ([]Run, error) {
	if _, err := os.Stat(dataDir); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, WeightsDirName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Run{}, nil
		}
		return nil, err
	}

	runs := make([]Run, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		run, err := scanRun(dataDir, entry.Name())
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	sort.Slice(runs, func(i, j int) bool { return runs[i].RunID < runs[j].RunID })
	return runs, nil
}

func scanRun(dataDir, runID string) (Run, error) {
	run := Run{RunID: runID, Checkpoints: []Checkpoint{}}

	entries, err := os.ReadDir(filepath.Join(dataDir, WeightsDirName, runID))
	if err != nil {
		return Run{}, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := checkpointPattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		episode, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		run.Checkpoints = append(run.Checkpoints, Checkpoint{
			Filename:   entry.Name(),
			Episode:    episode,
			ModifiedAt: formatTime(info.ModTime()),
		})
	}

	sort.Slice(run.Checkpoints, func(i, j int) bool {
		return run.Checkpoints[i].Episode < run.Checkpoints[j].Episode
	})

	if info, err := os.Stat(metrics.RunFile(dataDir, runID)); err == nil && !info.IsDir() {
		modified := formatTime(info.ModTime())
		run.HasMetrics = true
		run.MetricsModifiedAt = &modified
	}

	return run, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := ScanRuns(s.dataDir)
	if err != nil {
		s.logger.Error("failed to scan data directory", "data_dir", s.dataDir, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{apiError{
			Code:    errCodeDataDir,
			Message: "data directory does not exist or is not readable",
		}})
		return
	}
	writeJSON(w, http.StatusOK, RunsResponse{Runs: runs})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
