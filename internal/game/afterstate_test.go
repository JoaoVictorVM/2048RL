package game

import (
	"math/rand"
	"testing"
)

func TestAfterstate_MatchesApplyMoveMergeStep(t *testing.T) {
	start := Board{
		{2, 2, 4, 0},
		{0, 4, 4, 8},
		{2, 0, 0, 2},
		{16, 16, 0, 0},
	}

	for _, dir := range AllDirections {
		afterBoard, afterGained, afterMoved := Afterstate(start, dir)

		g := NewGame(WithBoard(start), WithRNG(rand.New(rand.NewSource(1))))
		before := g.Board()
		result := g.ApplyMove(dir)

		if result.Moved != afterMoved {
			t.Fatalf("%s: moved mismatch, Afterstate=%v ApplyMove=%v", dir, afterMoved, result.Moved)
		}
		if !result.Moved {
			if afterBoard != before {
				t.Errorf("%s: invalid move should leave the board untouched", dir)
			}
			continue
		}

		if result.ScoreGained != afterGained {
			t.Errorf("%s: score gained mismatch, Afterstate=%d ApplyMove=%d", dir, afterGained, result.ScoreGained)
		}

		stripped := g.Board()
		if result.Spawned {
			stripped[result.SpawnedCell.Row][result.SpawnedCell.Col] = 0
		}
		if stripped != afterBoard {
			t.Errorf("%s: board mismatch\nAfterstate: %v\nApplyMove (spawn stripped): %v", dir, afterBoard, stripped)
		}
	}
}

func TestAfterstate_DoesNotMutateInput(t *testing.T) {
	start := Board{
		{2, 2, 0, 0},
		{4, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	original := start

	if _, _, moved := Afterstate(start, Left); !moved {
		t.Fatal("expected the move to be valid")
	}
	if start != original {
		t.Errorf("Afterstate mutated its input: %v", start)
	}
}

func TestAfterstate_DeadBoardReportsNoMove(t *testing.T) {
	dead := Board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}

	for _, dir := range AllDirections {
		if _, _, moved := Afterstate(dead, dir); moved {
			t.Errorf("%s: expected no valid move on a dead board", dir)
		}
	}
}
