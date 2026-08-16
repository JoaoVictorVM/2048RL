package game

import (
	"reflect"
	"testing"
)

func TestNewGame_StartsWithTwoTiles(t *testing.T) {
	g := NewGame(WithSeed(1))
	filled := Size*Size - len(emptyCells(g.Board()))
	if filled != initialTiles {
		t.Errorf("fresh game has %d tiles, want %d", filled, initialTiles)
	}
	if g.Score() != 0 {
		t.Errorf("fresh game score = %d, want 0", g.Score())
	}
}

func TestNewGame_DefaultsToTimeSeededRNG(t *testing.T) {
	g := NewGame()
	if filled := Size*Size - len(emptyCells(g.Board())); filled != initialTiles {
		t.Errorf("fresh game has %d tiles, want %d", filled, initialTiles)
	}
}

func TestDirection_String(t *testing.T) {
	tests := map[Direction]string{
		Up:            "up",
		Down:          "down",
		Left:          "left",
		Right:         "right",
		Direction(99): "unknown",
	}
	for dir, want := range tests {
		if got := dir.String(); got != want {
			t.Errorf("Direction(%d).String() = %q, want %q", dir, got, want)
		}
	}
}

func TestValidMoves_MatchesManuallyVerifiedFixtures(t *testing.T) {
	tests := []struct {
		name  string
		board Board
		want  []Direction
	}{
		{
			name: "single tile in top-left corner",
			board: Board{
				{2, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
			},
			want: []Direction{Down, Right},
		},
		{
			name: "full board with one horizontal pair",
			board: Board{
				{2, 2, 8, 16},
				{32, 64, 128, 256},
				{2, 4, 8, 16},
				{32, 64, 128, 256},
			},
			want: []Direction{Left, Right},
		},
		{
			name: "dead board",
			board: Board{
				{2, 4, 2, 4},
				{4, 2, 4, 2},
				{2, 4, 2, 4},
				{4, 2, 4, 2},
			},
			want: []Direction{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGame(WithBoard(tt.board), WithSeed(1))
			got := g.ValidMoves()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidMoves() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsGameOver_TrueOnlyOnKnownDeadBoard(t *testing.T) {
	dead := Board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}
	if g := NewGame(WithBoard(dead), WithSeed(1)); !g.IsGameOver() {
		t.Error("expected the dead board to report game over")
	}

	withEmptyCell := dead
	withEmptyCell[3][3] = 0
	if g := NewGame(WithBoard(withEmptyCell), WithSeed(1)); g.IsGameOver() {
		t.Error("board with an empty cell must not report game over")
	}

	withAdjacentPair := dead
	withAdjacentPair[3][3] = withAdjacentPair[3][2]
	if g := NewGame(WithBoard(withAdjacentPair), WithSeed(1)); g.IsGameOver() {
		t.Error("board with an adjacent equal pair must not report game over")
	}
}

func TestIsWon_TrueOnFirst2048AndGameContinues(t *testing.T) {
	board := Board{
		{1024, 1024, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{4, 0, 0, 0},
	}
	g := NewGame(WithBoard(board), WithSeed(5))
	if g.IsWon() {
		t.Fatal("game reported won before the 2048 tile appeared")
	}

	result := g.ApplyMove(Left)
	if !result.Moved {
		t.Fatal("expected the merging move to be valid")
	}
	if !result.Won || !g.IsWon() {
		t.Fatalf("expected won after reaching 2048, got result.Won=%v IsWon=%v", result.Won, g.IsWon())
	}
	if g.MaxTile() != WinningTile {
		t.Errorf("MaxTile() = %d, want %d", g.MaxTile(), WinningTile)
	}
	if result.GameOver {
		t.Fatal("winning must not end the episode")
	}

	valid := g.ValidMoves()
	if len(valid) == 0 {
		t.Fatal("expected further valid moves after winning")
	}
	if next := g.ApplyMove(valid[0]); !next.Moved {
		t.Error("expected a further move to still be accepted after winning")
	}
}

func TestInvalidMove_RejectedWithNoStateChange(t *testing.T) {
	board := Board{
		{2, 4, 8, 16},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	g := NewGame(WithBoard(board), WithSeed(3), WithScore(120))

	result := g.ApplyMove(Up)
	if result.Moved {
		t.Fatal("expected the move to be rejected as invalid")
	}
	if result.Spawned {
		t.Error("an invalid move must not spawn a tile")
	}
	if result.ScoreGained != 0 {
		t.Errorf("ScoreGained = %d, want 0", result.ScoreGained)
	}
	if g.Board() != board {
		t.Errorf("board changed on an invalid move: %v", g.Board())
	}
	if g.Score() != 120 {
		t.Errorf("score = %d, want 120", g.Score())
	}
}

func TestApplyMove_SupportsAfterstateSimulationWithoutMutatingOriginal(t *testing.T) {
	g := NewGame(WithSeed(99))
	before := g.Board()
	beforeScore := g.Score()

	for _, dir := range AllDirections {
		snapshot := g.Board()
		afterstate, gained, moved := applyMoveToBoard(snapshot, dir)
		sim := NewGame(WithBoard(snapshot), WithSeed(1))
		simResult := sim.ApplyMove(dir)

		if simResult.Moved != moved {
			t.Errorf("%s: simulated Moved=%v, want %v", dir, simResult.Moved, moved)
		}
		if simResult.ScoreGained != gained {
			t.Errorf("%s: simulated ScoreGained=%d, want %d", dir, simResult.ScoreGained, gained)
		}
		if moved {
			simBoard := sim.Board()
			if simResult.Spawned {
				simBoard[simResult.SpawnedCell.Row][simResult.SpawnedCell.Col] = afterstate[simResult.SpawnedCell.Row][simResult.SpawnedCell.Col]
			}
			if simBoard != afterstate {
				t.Errorf("%s: simulated board %v differs from afterstate %v", dir, simBoard, afterstate)
			}
		}
	}

	if g.Board() != before || g.Score() != beforeScore {
		t.Error("simulating candidate moves mutated the live game")
	}
}

func TestApplyMove_IdenticalOutcomeForEquivalentInputs(t *testing.T) {
	board := Board{
		{2, 2, 4, 4},
		{8, 0, 8, 0},
		{0, 16, 0, 16},
		{32, 32, 2, 0},
	}

	a := NewGame(WithBoard(board), WithSeed(2024))
	b := NewGame(WithBoard(board), WithSeed(2024))

	resultA := a.ApplyMove(Left)
	resultB := b.ApplyMove(Left)

	if resultA != resultB {
		t.Errorf("move results differ: %+v vs %+v", resultA, resultB)
	}
	if a.Board() != b.Board() {
		t.Errorf("boards differ:\n%v\n%v", a.Board(), b.Board())
	}
	if a.Score() != b.Score() {
		t.Errorf("scores differ: %d vs %d", a.Score(), b.Score())
	}
}

func TestGame_PlaysFullEpisodeToGameOver(t *testing.T) {
	g := NewGame(WithSeed(2026))
	moves := 0
	for !g.IsGameOver() {
		valid := g.ValidMoves()
		if len(valid) == 0 {
			t.Fatal("IsGameOver() is false but no valid moves remain")
		}
		result := g.ApplyMove(valid[0])
		if !result.Moved {
			t.Fatalf("move %v reported valid but did not change the board", valid[0])
		}
		moves++
		if moves > 100000 {
			t.Fatal("episode did not terminate")
		}
	}
	if moves == 0 {
		t.Fatal("expected at least one move before game over")
	}
	if g.Score() <= 0 {
		t.Errorf("score = %d, want > 0 after a full episode", g.Score())
	}
}
