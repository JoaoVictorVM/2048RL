package web

import (
	"errors"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/JoaoVictorVM/2048RL/internal/metrics"
)

const (
	DefaultMetricsWindow = 100

	errCodeUnknownRun = "DASH001"
)

type metricsPoint struct {
	WindowStart int     `json:"window_start"`
	WindowEnd   int     `json:"window_end"`
	AvgScore    float64 `json:"avg_score"`
	AvgMaxTile  float64 `json:"avg_max_tile"`
	WinRate     float64 `json:"win_rate"`
}

type runMetricsResponse struct {
	RunID        string         `json:"run_id"`
	WindowSize   int            `json:"window_size"`
	EpisodeCount int            `json:"episode_count"`
	Points       []metricsPoint `json:"points"`
}

func (s *Server) handleRunMetrics(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")

	runs, err := ScanRuns(s.dataDir)
	if err != nil {
		s.logger.Warn("failed to scan data directory", "data_dir", s.dataDir, "error", err)
	}
	if _, ok := findRunByID(runs, runID); !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{apiError{
			Code:    errCodeUnknownRun,
			Message: "run " + runID + " não encontrado",
		}})
		return
	}

	window := DefaultMetricsWindow
	if raw := r.URL.Query().Get("window"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			window = parsed
		}
	}

	records, err := metrics.ReadAll(metrics.RunFile(s.dataDir, runID))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.logger.Warn("failed to read run metrics", "run_id", runID, "error", err)
	}

	writeJSON(w, http.StatusOK, runMetricsResponse{
		RunID:        runID,
		WindowSize:   window,
		EpisodeCount: len(records),
		Points:       windowPoints(records, window),
	})
}

// A última janela pode ficar incompleta: ela entra assim mesmo, com o tamanho
// que sobrou, para o gráfico refletir todo o histórico do run.
func windowPoints(records []metrics.Record, window int) []metricsPoint {
	points := make([]metricsPoint, 0, len(records)/window+1)

	for start := 0; start < len(records); start += window {
		end := start + window
		if end > len(records) {
			end = len(records)
		}

		chunk := records[start:end]
		summary := metrics.Summarize(chunk)
		points = append(points, metricsPoint{
			WindowStart: chunk[0].Episode,
			WindowEnd:   chunk[len(chunk)-1].Episode,
			AvgScore:    summary.AvgScore,
			AvgMaxTile:  summary.AvgMaxTile,
			WinRate:     summary.WinRate,
		})
	}
	return points
}
