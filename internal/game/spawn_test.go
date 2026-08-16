package game

import (
	"math/rand"
	"testing"
)

func TestSpawn_AlwaysLandsOnEmptyCell(t *testing.T) {
	boards := []Board{
		{},
		{
			{2, 4, 8, 16},
			{0, 0, 0, 0},
			{2, 4, 8, 16},
			{32, 64, 128, 0},
		},
		{
			{2, 4, 8, 16},
			{32, 64, 128, 256},
			{2, 4, 8, 16},
			{32, 64, 128, 0},
		},
	}

	rng := rand.New(rand.NewSource(7))
	for i, base := range boards {
		for trial := 0; trial < 500; trial++ {
			b := base
			cell, value, ok := spawnTile(&b, rng)
			if !ok {
				t.Fatalf("board %d: spawn reported no room on a board with empty cells", i)
			}
			if base[cell.Row][cell.Col] != 0 {
				t.Fatalf("board %d: spawned into occupied cell %v", i, cell)
			}
			if b[cell.Row][cell.Col] != value {
				t.Fatalf("board %d: cell %v holds %d, want spawned value %d", i, cell, b[cell.Row][cell.Col], value)
			}
		}
	}
}

func TestSpawn_NoRoomOnFullBoard(t *testing.T) {
	b := Board{
		{2, 4, 8, 16},
		{32, 64, 128, 256},
		{2, 4, 8, 16},
		{32, 64, 128, 256},
	}
	if _, _, ok := spawnTile(&b, rand.New(rand.NewSource(1))); ok {
		t.Error("expected spawn to fail on a full board")
	}
}

func TestSpawn_ApproximatesNinetyTenDistribution(t *testing.T) {
	const trials = 10000
	rng := rand.New(rand.NewSource(42))

	twos := 0
	for i := 0; i < trials; i++ {
		var b Board
		_, value, ok := spawnTile(&b, rng)
		if !ok {
			t.Fatal("spawn failed on an empty board")
		}
		switch value {
		case 2:
			twos++
		case 4:
		default:
			t.Fatalf("unexpected spawned value %d", value)
		}
	}

	ratio := float64(twos) / float64(trials)
	if ratio < 0.88 || ratio > 0.92 {
		t.Errorf("value-2 ratio = %.4f, want within [0.88, 0.92]", ratio)
	}
}

func TestSpawn_DeterministicWithSeededRNG(t *testing.T) {
	const moves = 40
	collect := func() []MoveResult {
		g := NewGame(WithSeed(123))
		results := make([]MoveResult, 0, moves)
		for i := 0; i < moves; i++ {
			valid := g.ValidMoves()
			if len(valid) == 0 {
				break
			}
			results = append(results, g.ApplyMove(valid[0]))
		}
		return results
	}

	first, second := collect(), collect()
	if len(first) != len(second) {
		t.Fatalf("sequence lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("move %d differs: %+v vs %+v", i, first[i], second[i])
		}
	}
}
