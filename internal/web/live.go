package web

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/agent"
	"github.com/JoaoVictorVM/2048RL/internal/game"
)

const (
	DefaultMoveDelay           = 300 * time.Millisecond
	DefaultEpisodeRestartDelay = 2 * time.Second

	errCodeCheckpointNotFound = "LIVE001"
	errCodeNoCheckpoints      = "LIVE002"
	errCodeCheckpointLoad     = "LIVE003"
)

type livePacing struct {
	moveDelay    time.Duration
	restartDelay time.Duration
}

type checkpointRef struct {
	RunID    string
	Filename string
	Path     string
}

type liveErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type liveEpisodeStartMessage struct {
	Type       string    `json:"type"`
	RunID      string    `json:"run_id"`
	Checkpoint string    `json:"checkpoint"`
	Board      boardJSON `json:"board"`
	Score      int       `json:"score"`
}

type liveMoveMessage struct {
	Type      string    `json:"type"`
	Board     boardJSON `json:"board"`
	Score     int       `json:"score"`
	MoveCount int       `json:"move_count"`
	Direction string    `json:"direction"`
	GameOver  bool      `json:"game_over"`
	Won       bool      `json:"won"`
}

type liveEpisodeEndMessage struct {
	Type      string `json:"type"`
	Score     int    `json:"score"`
	MaxTile   int    `json:"max_tile"`
	Won       bool   `json:"won"`
	MoveCount int    `json:"move_count"`
}

func (s *Server) handleLiveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrade(w, r)
	if err != nil {
		s.logger.Warn("live websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	ref, liveErr := resolveCheckpoint(s.dataDir, r.URL.Query().Get("run_id"), r.URL.Query().Get("checkpoint"))
	if liveErr != nil {
		s.logger.Info("live stream rejected", "code", liveErr.Code, "message", liveErr.Message)
		_ = conn.WriteJSON(liveErr)
		return
	}

	network, err := agent.LoadNetwork(s.agentCfg, ref.Path)
	if err != nil {
		s.logger.Error("failed to load checkpoint for live stream", "path", ref.Path, "error", err)
		_ = conn.WriteJSON(liveErrorMessage{
			Type:    "error",
			Code:    errCodeCheckpointLoad,
			Message: "não foi possível carregar o checkpoint " + ref.Filename,
		})
		return
	}

	s.streamEpisodes(conn, ref, network)
}

func resolveCheckpoint(dataDir, runID, filename string) (checkpointRef, *liveErrorMessage) {
	runs, err := ScanRuns(dataDir)
	if err != nil {
		return checkpointRef{}, &liveErrorMessage{
			Type:    "error",
			Code:    errCodeNoCheckpoints,
			Message: "nenhum checkpoint disponível ainda",
		}
	}

	if runID != "" {
		run, ok := findRunByID(runs, runID)
		if !ok {
			return checkpointRef{}, notFoundError("run " + runID + " não encontrado")
		}
		runs = []Run{run}
	}

	best, ok := latestCheckpoint(runs, filename)
	if !ok {
		if runID != "" || filename != "" {
			return checkpointRef{}, notFoundError("checkpoint não encontrado para a seleção informada")
		}
		return checkpointRef{}, &liveErrorMessage{
			Type:    "error",
			Code:    errCodeNoCheckpoints,
			Message: "nenhum checkpoint disponível ainda",
		}
	}

	best.Path = filepath.Join(dataDir, WeightsDirName, best.RunID, best.Filename)
	return best, nil
}

func notFoundError(message string) *liveErrorMessage {
	return &liveErrorMessage{Type: "error", Code: errCodeCheckpointNotFound, Message: message}
}

func findRunByID(runs []Run, runID string) (Run, bool) {
	for _, run := range runs {
		if run.RunID == runID {
			return run, true
		}
	}
	return Run{}, false
}

func latestCheckpoint(runs []Run, filename string) (checkpointRef, bool) {
	var best checkpointRef
	var bestModified time.Time
	var bestEpisode int
	found := false

	for _, run := range runs {
		for _, checkpoint := range run.Checkpoints {
			if filename != "" && checkpoint.Filename != filename {
				continue
			}
			modified, err := time.Parse(time.RFC3339, checkpoint.ModifiedAt)
			if err != nil {
				continue
			}
			better := !found || modified.After(bestModified) ||
				(modified.Equal(bestModified) && checkpoint.Episode > bestEpisode)
			if !better {
				continue
			}
			best = checkpointRef{RunID: run.RunID, Filename: checkpoint.Filename}
			bestModified, bestEpisode, found = modified, checkpoint.Episode, true
		}
	}
	return best, found
}

func (s *Server) streamEpisodes(conn *Conn, ref checkpointRef, network *agent.Network) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		if !s.streamEpisode(conn, done, ref, network) {
			return
		}
		if !waitOrDone(done, s.live.restartDelay) {
			return
		}
	}
}

func (s *Server) streamEpisode(conn *Conn, done <-chan struct{}, ref checkpointRef, network *agent.Network) bool {
	g := game.NewGame()

	if err := conn.WriteJSON(liveEpisodeStartMessage{
		Type:       "episode_start",
		RunID:      ref.RunID,
		Checkpoint: ref.Filename,
		Board:      boardJSON(g.Board()),
		Score:      g.Score(),
	}); err != nil {
		return false
	}

	moveCount := 0
	for {
		direction, _, ok := network.SelectMove(g)
		if !ok {
			break
		}

		result := g.ApplyMove(direction)
		moveCount++

		if err := conn.WriteJSON(liveMoveMessage{
			Type:      "move",
			Board:     boardJSON(g.Board()),
			Score:     g.Score(),
			MoveCount: moveCount,
			Direction: direction.String(),
			GameOver:  result.GameOver,
			Won:       result.Won,
		}); err != nil {
			return false
		}

		if result.GameOver {
			break
		}
		if !waitOrDone(done, s.live.moveDelay) {
			return false
		}
	}

	return conn.WriteJSON(liveEpisodeEndMessage{
		Type:      "episode_end",
		Score:     g.Score(),
		MaxTile:   g.MaxTile(),
		Won:       g.IsWon(),
		MoveCount: moveCount,
	}) == nil
}

func waitOrDone(done <-chan struct{}, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-done:
		return false
	case <-timer.C:
		return true
	}
}
