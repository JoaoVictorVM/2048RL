package web

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

const (
	errCodeInvalidDirection = "HUMAN001"
	errCodeUnknownSession   = "HUMAN002"

	maxMoveBodyBytes = 1 << 12
)

var directionsByName = map[string]game.Direction{
	"up":    game.Up,
	"down":  game.Down,
	"left":  game.Left,
	"right": game.Right,
}

type newSessionResponse struct {
	SessionID string    `json:"session_id"`
	Board     boardJSON `json:"board"`
	Score     int       `json:"score"`
	GameOver  bool      `json:"game_over"`
	Won       bool      `json:"won"`
}

type moveRequest struct {
	SessionID string `json:"session_id"`
	Direction string `json:"direction"`
}

type moveResponse struct {
	Board    boardJSON `json:"board"`
	Score    int       `json:"score"`
	Moved    bool      `json:"moved"`
	GameOver bool      `json:"game_over"`
	Won      bool      `json:"won"`
}

type referenceResponse struct {
	Available  bool    `json:"available"`
	RunID      string  `json:"run_id,omitempty"`
	AvgScore   float64 `json:"avg_score,omitempty"`
	AvgMaxTile float64 `json:"avg_max_tile,omitempty"`
}

type boardJSON [game.Size][game.Size]int

type metricsRecord struct {
	Episode int  `json:"episode"`
	Score   int  `json:"score"`
	MaxTile int  `json:"max_tile"`
	Won     bool `json:"won"`
	Moves   int  `json:"moves"`
}

func (s *Server) handleHumanNew(w http.ResponseWriter, r *http.Request) {
	session, err := s.sessions.Create(s.newGame())
	if err != nil {
		s.logger.Error("failed to create human play session", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{apiError{
			Code:    errCodeUnknownSession,
			Message: "não foi possível criar a sessão",
		}})
		return
	}

	g := session.Game
	writeJSON(w, http.StatusCreated, newSessionResponse{
		SessionID: session.Token,
		Board:     boardJSON(g.Board()),
		Score:     g.Score(),
		GameOver:  g.IsGameOver(),
		Won:       g.IsWon(),
	})
}

func (s *Server) handleHumanMove(w http.ResponseWriter, r *http.Request) {
	var req moveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxMoveBodyBytes)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{apiError{
			Code:    errCodeInvalidDirection,
			Message: "corpo da requisição inválido",
		}})
		return
	}

	direction, ok := directionsByName[strings.ToLower(strings.TrimSpace(req.Direction))]
	if !ok {
		writeJSON(w, http.StatusBadRequest, errorResponse{apiError{
			Code:    errCodeInvalidDirection,
			Message: "direção deve ser up, down, left ou right",
		}})
		return
	}

	session, ok := s.sessions.Get(req.SessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{apiError{
			Code:    errCodeUnknownSession,
			Message: "sessão desconhecida ou expirada",
		}})
		return
	}

	session.Lock()
	defer session.Unlock()

	result := session.Game.ApplyMove(direction)
	writeJSON(w, http.StatusOK, moveResponse{
		Board:    boardJSON(session.Game.Board()),
		Score:    session.Game.Score(),
		Moved:    result.Moved,
		GameOver: result.GameOver,
		Won:      result.Won,
	})
}

func (s *Server) handleHumanReference(w http.ResponseWriter, r *http.Request) {
	reference, err := readReferenceStat(s.dataDir)
	if err != nil {
		s.logger.Warn("failed to read reference metrics", "data_dir", s.dataDir, "error", err)
		writeJSON(w, http.StatusOK, referenceResponse{Available: false})
		return
	}
	writeJSON(w, http.StatusOK, reference)
}

func readReferenceStat(dataDir string) (referenceResponse, error) {
	runID, path, err := mostRecentMetricsFile(dataDir)
	if err != nil {
		return referenceResponse{}, err
	}
	if path == "" {
		return referenceResponse{Available: false}, nil
	}

	records, err := readMetricsRecords(path)
	if err != nil {
		return referenceResponse{}, err
	}
	if len(records) == 0 {
		return referenceResponse{Available: false}, nil
	}

	totalScore, totalMaxTile := 0, 0
	for _, record := range records {
		totalScore += record.Score
		totalMaxTile += record.MaxTile
	}

	count := float64(len(records))
	return referenceResponse{
		Available:  true,
		RunID:      runID,
		AvgScore:   float64(totalScore) / count,
		AvgMaxTile: float64(totalMaxTile) / count,
	}, nil
}

func mostRecentMetricsFile(dataDir string) (string, string, error) {
	entries, err := os.ReadDir(filepath.Join(dataDir, metricsDirName))
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
		candidate := filepath.Join(dataDir, metricsDirName, entry.Name(), metricsFile)
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

func readMetricsRecords(path string) ([]metricsRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []metricsRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record metricsRecord
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
