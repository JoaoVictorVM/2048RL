package agent

import "github.com/JoaoVictorVM/2048RL/internal/game"

const SymmetryCount = 8

func rotate(b game.Board) game.Board {
	var out game.Board
	for r := 0; r < game.Size; r++ {
		for c := 0; c < game.Size; c++ {
			out[r][c] = b[game.Size-1-c][r]
		}
	}
	return out
}

func mirror(b game.Board) game.Board {
	var out game.Board
	for r := 0; r < game.Size; r++ {
		for c := 0; c < game.Size; c++ {
			out[r][c] = b[r][game.Size-1-c]
		}
	}
	return out
}

func symmetries(b game.Board) [SymmetryCount]game.Board {
	var out [SymmetryCount]game.Board

	current := b
	for i := 0; i < 4; i++ {
		out[i] = current
		out[i+4] = mirror(current)
		current = rotate(current)
	}
	return out
}
