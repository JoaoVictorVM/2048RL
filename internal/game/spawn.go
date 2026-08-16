package game

import "math/rand"

const spawnFourChance = 10

func emptyCells(b Board) []Cell {
	cells := make([]Cell, 0, Size*Size)
	for r := 0; r < Size; r++ {
		for c := 0; c < Size; c++ {
			if b[r][c] == 0 {
				cells = append(cells, Cell{Row: r, Col: c})
			}
		}
	}
	return cells
}

func spawnTile(b *Board, rng *rand.Rand) (Cell, int, bool) {
	cells := emptyCells(*b)
	if len(cells) == 0 {
		return Cell{}, 0, false
	}

	cell := cells[rng.Intn(len(cells))]
	value := 2
	if rng.Intn(100) < spawnFourChance {
		value = 4
	}
	b[cell.Row][cell.Col] = value
	return cell, value, true
}
