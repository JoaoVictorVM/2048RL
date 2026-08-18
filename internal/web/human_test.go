package web

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/game"
	"github.com/JoaoVictorVM/2048RL/internal/metrics"
)

const humanTestSeed = 2026

func fixedGameFactory(board game.Board) func() *game.Game {
	return func() *game.Game {
		return game.NewGame(game.WithBoard(board), game.WithSeed(humanTestSeed))
	}
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()

	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		payload = encoded
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func startSession(t *testing.T, baseURL string) newSessionResponse {
	t.Helper()

	resp := postJSON(t, baseURL+"/api/human/new", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d", resp.StatusCode)
	}

	var payload newSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload
}

func applyMove(t *testing.T, baseURL, sessionID, direction string) (*http.Response, moveResponse) {
	t.Helper()

	resp := postJSON(t, baseURL+"/api/human/move", moveRequest{SessionID: sessionID, Direction: direction})
	t.Cleanup(func() { resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		return resp, moveResponse{}
	}

	var payload moveResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp, payload
}

func decodeError(t *testing.T, resp *http.Response) apiError {
	t.Helper()

	var payload errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload.Error
}

func countTiles(b boardJSON) (count int, values []int) {
	for r := 0; r < game.Size; r++ {
		for c := 0; c < game.Size; c++ {
			if b[r][c] != 0 {
				count++
				values = append(values, b[r][c])
			}
		}
	}
	return count, values
}

func TestHumanNew_ReturnsFreshBoard(t *testing.T) {
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir()})

	session := startSession(t, ts.URL)

	if session.SessionID == "" {
		t.Error("esperado um session_id não vazio")
	}
	if session.Score != 0 {
		t.Errorf("esperado score 0, obtido %d", session.Score)
	}
	if session.GameOver || session.Won {
		t.Errorf("sessão nova não deveria estar encerrada nem vencida: %+v", session)
	}

	count, values := countTiles(session.Board)
	if count != 2 {
		t.Errorf("esperados 2 tiles iniciais, obtidos %d", count)
	}
	for _, value := range values {
		if value != 2 && value != 4 {
			t.Errorf("tile inicial inválido: %d", value)
		}
	}
}

func TestHumanNew_CreatesIndependentSessions(t *testing.T) {
	board := game.Board{
		{2, 2, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), NewGame: fixedGameFactory(board)})

	first := startSession(t, ts.URL)
	second := startSession(t, ts.URL)

	if first.SessionID == second.SessionID {
		t.Fatal("cada sessão deveria receber um token distinto")
	}

	if _, moved := applyMove(t, ts.URL, first.SessionID, "left"); moved.Score == 0 {
		t.Fatalf("esperado ganho de score na primeira sessão: %+v", moved)
	}

	_, untouched := applyMove(t, ts.URL, second.SessionID, "up")
	if untouched.Score != 0 {
		t.Errorf("a segunda sessão não deveria ser afetada pela primeira: %+v", untouched)
	}
}

func TestHumanMove_AppliesSameRulesAsEngine(t *testing.T) {
	board := game.Board{
		{2, 2, 4, 0},
		{0, 4, 4, 8},
		{2, 0, 0, 2},
		{16, 16, 0, 0},
	}
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), NewGame: fixedGameFactory(board)})

	session := startSession(t, ts.URL)
	local := game.NewGame(game.WithBoard(board), game.WithSeed(humanTestSeed))

	if boardJSON(local.Board()) != session.Board {
		t.Fatalf("tabuleiro inicial divergente\nendpoint: %v\nmotor:    %v", session.Board, local.Board())
	}

	for i, direction := range []string{"left", "up", "right", "down", "left", "left"} {
		_, response := applyMove(t, ts.URL, session.SessionID, direction)
		expected := local.ApplyMove(directionsByName[direction])

		if response.Moved != expected.Moved {
			t.Fatalf("jogada %d (%s): moved %v, esperado %v", i, direction, response.Moved, expected.Moved)
		}
		if boardJSON(local.Board()) != response.Board {
			t.Fatalf("jogada %d (%s): tabuleiro divergente\nendpoint: %v\nmotor:    %v",
				i, direction, response.Board, local.Board())
		}
		if response.Score != local.Score() {
			t.Fatalf("jogada %d (%s): score %d, esperado %d", i, direction, response.Score, local.Score())
		}
		if response.GameOver != local.IsGameOver() || response.Won != local.IsWon() {
			t.Fatalf("jogada %d (%s): flags divergentes %+v", i, direction, response)
		}
	}
}

func TestHumanMove_NoOpDirectionLeavesStateUnchanged(t *testing.T) {
	board := game.Board{
		{2, 4, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), NewGame: fixedGameFactory(board)})

	session := startSession(t, ts.URL)

	resp, response := applyMove(t, ts.URL, session.SessionID, "left")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("uma jogada inócua deveria responder 200, obtido %d", resp.StatusCode)
	}
	if response.Moved {
		t.Error("esperado moved=false para uma jogada que não altera o tabuleiro")
	}
	if response.Board != session.Board {
		t.Errorf("o tabuleiro não deveria mudar: %v", response.Board)
	}
	if response.Score != 0 {
		t.Errorf("o score não deveria mudar, obtido %d", response.Score)
	}
	if count, _ := countTiles(response.Board); count != 2 {
		t.Errorf("nenhum tile deveria nascer numa jogada inócua, tiles: %d", count)
	}
}

func TestHumanMove_InvalidDirectionReturnsHUMAN001(t *testing.T) {
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir()})
	session := startSession(t, ts.URL)

	for _, direction := range []string{"diagonal", "", "UPWARDS"} {
		resp, _ := applyMove(t, ts.URL, session.SessionID, direction)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("direção %q: esperado 400, obtido %d", direction, resp.StatusCode)
		}
		if code := decodeError(t, resp).Code; code != errCodeInvalidDirection {
			t.Errorf("direção %q: esperado código %s, obtido %s", direction, errCodeInvalidDirection, code)
		}
	}
}

func TestHumanMove_MalformedBodyReturnsHUMAN001(t *testing.T) {
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir()})

	resp, err := http.Post(ts.URL+"/api/human/move", "application/json", bytes.NewReader([]byte("{isso não é json")))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperado 400, obtido %d", resp.StatusCode)
	}
	if code := decodeError(t, resp).Code; code != errCodeInvalidDirection {
		t.Errorf("esperado código %s, obtido %s", errCodeInvalidDirection, code)
	}
}

func TestHumanMove_UnknownSessionReturnsHUMAN002(t *testing.T) {
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir()})

	resp, _ := applyMove(t, ts.URL, "5f2c1a9e-7b3d-4e21-9c0a-1a2b3c4d5e6f", "left")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("esperado 404, obtido %d", resp.StatusCode)
	}
	if code := decodeError(t, resp).Code; code != errCodeUnknownSession {
		t.Errorf("esperado código %s, obtido %s", errCodeUnknownSession, code)
	}
}

func TestHumanMove_ExpiredSessionReturnsHUMAN002(t *testing.T) {
	srv := NewServer(Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), Logger: discardLogger()})
	now := time.Now()
	srv.sessions.now = func() time.Time { return now }

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	session := startSession(t, ts.URL)
	now = now.Add(DefaultSessionTTL + time.Minute)

	resp, _ := applyMove(t, ts.URL, session.SessionID, "left")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("esperado 404 para sessão expirada, obtido %d", resp.StatusCode)
	}
	if code := decodeError(t, resp).Code; code != errCodeUnknownSession {
		t.Errorf("esperado código %s, obtido %s", errCodeUnknownSession, code)
	}
}

func TestHumanMove_ReportsGameOverOnDeadBoard(t *testing.T) {
	board := game.Board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), NewGame: fixedGameFactory(board)})

	session := startSession(t, ts.URL)
	if !session.GameOver {
		t.Fatalf("a sessão deveria nascer em game over: %+v", session)
	}

	_, response := applyMove(t, ts.URL, session.SessionID, "down")
	if response.Moved {
		t.Error("nenhuma direção deveria ser válida no tabuleiro morto")
	}
	if !response.GameOver {
		t.Errorf("esperado game_over=true: %+v", response)
	}
}

func TestHumanMove_ReportsWonWhenTileReaches2048(t *testing.T) {
	board := game.Board{
		{1024, 1024, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir(), NewGame: fixedGameFactory(board)})

	session := startSession(t, ts.URL)
	if session.Won {
		t.Fatal("a sessão não deveria nascer vencida")
	}

	_, response := applyMove(t, ts.URL, session.SessionID, "left")
	if !response.Won {
		t.Errorf("esperado won=true após formar 2048: %+v", response)
	}
	if response.GameOver {
		t.Error("a partida deveria continuar após atingir 2048")
	}
}

func writeMetricsRecords(t *testing.T, dataDir, runID string, records []metrics.Record, modTime time.Time) {
	t.Helper()

	path := metrics.RunFile(dataDir, runID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var buf bytes.Buffer
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

func getReference(t *testing.T, baseURL string) referenceResponse {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/human/reference")
	if err != nil {
		t.Fatalf("GET /api/human/reference: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", resp.StatusCode)
	}

	var payload referenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload
}

func TestHumanReference_UnavailableWhenNoRunsExist(t *testing.T) {
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir()})

	if reference := getReference(t, ts.URL); reference.Available {
		t.Errorf("esperado available=false sem runs: %+v", reference)
	}
}

func TestHumanReference_UnavailableWhenDataDirMissing(t *testing.T) {
	ts := newTestServer(t, Config{DataDir: filepath.Join(t.TempDir(), "ausente"), StaticDir: t.TempDir()})

	if reference := getReference(t, ts.URL); reference.Available {
		t.Errorf("esperado available=false com diretório ausente: %+v", reference)
	}
}

func TestHumanReference_ReturnsStatsFromMostRecentRun(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now()

	writeMetricsRecords(t, dataDir, "run-antigo", []metrics.Record{
		{Episode: 1, Score: 100, MaxTile: 128},
		{Episode: 2, Score: 300, MaxTile: 256},
	}, now.Add(-2*time.Hour))

	writeMetricsRecords(t, dataDir, "run-recente", []metrics.Record{
		{Episode: 1, Score: 1000, MaxTile: 1024, Won: false, Moves: 400},
		{Episode: 2, Score: 3000, MaxTile: 2048, Won: true, Moves: 900},
		{Episode: 3, Score: 2000, MaxTile: 2048, Won: true, Moves: 700},
	}, now)

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})
	reference := getReference(t, ts.URL)

	if !reference.Available {
		t.Fatalf("esperado available=true: %+v", reference)
	}
	if reference.RunID != "run-recente" {
		t.Errorf("esperado o run mais recente, obtido %q", reference.RunID)
	}
	if reference.AvgScore != 2000 {
		t.Errorf("esperado avg_score 2000, obtido %v", reference.AvgScore)
	}
	if want := (1024.0 + 2048 + 2048) / 3; reference.AvgMaxTile != want {
		t.Errorf("esperado avg_max_tile %v, obtido %v", want, reference.AvgMaxTile)
	}
}

func TestHumanReference_SkipsMalformedRecords(t *testing.T) {
	dataDir := t.TempDir()
	path := metrics.RunFile(dataDir, "run-a")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := "{\"episode\":1,\"score\":100,\"max_tile\":128}\nlinha corrompida\n\n{\"episode\":2,\"score\":300,\"max_tile\":256}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})
	reference := getReference(t, ts.URL)

	if !reference.Available {
		t.Fatalf("esperado available=true: %+v", reference)
	}
	if reference.AvgScore != 200 {
		t.Errorf("esperado avg_score 200 ignorando a linha corrompida, obtido %v", reference.AvgScore)
	}
}

func TestHumanReference_UnavailableWhenMetricsFileIsEmpty(t *testing.T) {
	dataDir := t.TempDir()
	writeMetricsRecords(t, dataDir, "run-vazio", nil, time.Now())

	ts := newTestServer(t, Config{DataDir: dataDir, StaticDir: t.TempDir()})
	if reference := getReference(t, ts.URL); reference.Available {
		t.Errorf("esperado available=false com métricas vazias: %+v", reference)
	}
}

func TestHumanPlay_FullGameStaysConsistentWithEngine(t *testing.T) {
	ts := newTestServer(t, Config{DataDir: t.TempDir(), StaticDir: t.TempDir()})
	session := startSession(t, ts.URL)

	rng := rand.New(rand.NewSource(11))
	directions := []string{"up", "down", "left", "right"}

	current := session.Board
	score := session.Score
	for i := 0; i < 200; i++ {
		direction := directions[rng.Intn(len(directions))]
		_, response := applyMove(t, ts.URL, session.SessionID, direction)

		expectedBoard, gained, moved := game.Afterstate(game.Board(current), directionsByName[direction])
		if response.Moved != moved {
			t.Fatalf("jogada %d (%s): moved %v, esperado %v", i, direction, response.Moved, moved)
		}
		if !moved {
			if response.Board != current || response.Score != score {
				t.Fatalf("jogada %d (%s): estado mudou numa jogada inválida", i, direction)
			}
			continue
		}

		if response.Score != score+gained {
			t.Fatalf("jogada %d (%s): score %d, esperado %d", i, direction, response.Score, score+gained)
		}

		spawned := 0
		for r := 0; r < game.Size; r++ {
			for c := 0; c < game.Size; c++ {
				if response.Board[r][c] == expectedBoard[r][c] {
					continue
				}
				if expectedBoard[r][c] != 0 || (response.Board[r][c] != 2 && response.Board[r][c] != 4) {
					t.Fatalf("jogada %d (%s): célula (%d,%d) divergente do afterstate do motor", i, direction, r, c)
				}
				spawned++
			}
		}
		if spawned != 1 {
			t.Fatalf("jogada %d (%s): esperado exatamente 1 tile novo, obtidos %d", i, direction, spawned)
		}

		current = response.Board
		score = response.Score
		if response.GameOver {
			return
		}
	}
}
