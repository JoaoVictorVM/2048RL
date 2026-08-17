package agent

import (
	"testing"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

var asymmetricBoard = game.Board{
	{2, 4, 8, 16},
	{32, 64, 128, 256},
	{512, 1024, 2048, 4096},
	{8192, 16384, 32768, 0},
}

func TestSymmetry_AllEightTransformsAreDistinct(t *testing.T) {
	boards := symmetries(asymmetricBoard)

	for i := 0; i < len(boards); i++ {
		for j := i + 1; j < len(boards); j++ {
			if boards[i] == boards[j] {
				t.Errorf("transformações %d e %d produziram o mesmo tabuleiro", i, j)
			}
		}
	}
}

func TestSymmetry_IncludesTheOriginalBoard(t *testing.T) {
	boards := symmetries(asymmetricBoard)
	for _, b := range boards {
		if b == asymmetricBoard {
			return
		}
	}
	t.Error("o conjunto de simetrias deveria conter o tabuleiro original")
}

func TestSymmetry_TransformIsBijective(t *testing.T) {
	if got := mirror(mirror(asymmetricBoard)); got != asymmetricBoard {
		t.Error("espelhar duas vezes deveria devolver o tabuleiro original")
	}

	rotated := asymmetricBoard
	for i := 0; i < 4; i++ {
		rotated = rotate(rotated)
	}
	if rotated != asymmetricBoard {
		t.Error("rotacionar quatro vezes deveria devolver o tabuleiro original")
	}
}

func TestSymmetry_PreservesTileMultiset(t *testing.T) {
	count := func(b game.Board) map[int]int {
		tiles := map[int]int{}
		for r := 0; r < game.Size; r++ {
			for c := 0; c < game.Size; c++ {
				tiles[b[r][c]]++
			}
		}
		return tiles
	}

	want := count(asymmetricBoard)
	for i, b := range symmetries(asymmetricBoard) {
		got := count(b)
		if len(got) != len(want) {
			t.Fatalf("transformação %d alterou o conjunto de tiles", i)
		}
		for tile, n := range want {
			if got[tile] != n {
				t.Errorf("transformação %d: tile %d aparece %d vezes, esperado %d", i, tile, got[tile], n)
			}
		}
	}
}
