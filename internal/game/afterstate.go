package game

func Afterstate(b Board, dir Direction) (Board, int, bool) {
	return applyMoveToBoard(b, dir)
}
