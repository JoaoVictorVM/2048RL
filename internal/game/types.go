package game

const Size = 4

const WinningTile = 2048

type Board [Size][Size]int

type Direction int

const (
	Up Direction = iota
	Down
	Left
	Right
)

var AllDirections = [...]Direction{Up, Down, Left, Right}

func (d Direction) String() string {
	switch d {
	case Up:
		return "up"
	case Down:
		return "down"
	case Left:
		return "left"
	case Right:
		return "right"
	default:
		return "unknown"
	}
}

type Cell struct {
	Row int
	Col int
}

type MoveResult struct {
	Moved        bool
	ScoreGained  int
	Spawned      bool
	SpawnedCell  Cell
	SpawnedValue int
	GameOver     bool
	Won          bool
}
