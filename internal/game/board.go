package game

var cwRotationsForLeft = map[Direction]int{
	Left:  0,
	Down:  1,
	Right: 2,
	Up:    3,
}

func rotateCW(b Board) Board {
	var out Board
	for r := 0; r < Size; r++ {
		for c := 0; c < Size; c++ {
			out[r][c] = b[Size-1-c][r]
		}
	}
	return out
}

func rotateCWTimes(b Board, times int) Board {
	for i := 0; i < times%4; i++ {
		b = rotateCW(b)
	}
	return b
}

func slideRowLeft(row [Size]int) ([Size]int, int) {
	var compact [Size]int
	n := 0
	for _, v := range row {
		if v != 0 {
			compact[n] = v
			n++
		}
	}

	var out [Size]int
	gained := 0
	w := 0
	for i := 0; i < n; i++ {
		if i+1 < n && compact[i] == compact[i+1] {
			merged := compact[i] * 2
			out[w] = merged
			gained += merged
			w++
			i++ // pula o tile consumido: ele não pode se fundir de novo nesta jogada
			continue
		}
		out[w] = compact[i]
		w++
	}
	return out, gained
}

func applyMoveToBoard(b Board, dir Direction) (Board, int, bool) {
	turns := cwRotationsForLeft[dir]
	rotated := rotateCWTimes(b, turns)

	gained := 0
	for r := 0; r < Size; r++ {
		newRow, rowScore := slideRowLeft(rotated[r])
		rotated[r] = newRow
		gained += rowScore
	}

	result := rotateCWTimes(rotated, 4-turns)
	return result, gained, result != b
}

func canMove(b Board) bool {
	for _, d := range AllDirections {
		if _, _, moved := applyMoveToBoard(b, d); moved {
			return true
		}
	}
	return false
}

func maxTile(b Board) int {
	max := 0
	for r := 0; r < Size; r++ {
		for c := 0; c < Size; c++ {
			if b[r][c] > max {
				max = b[r][c]
			}
		}
	}
	return max
}
