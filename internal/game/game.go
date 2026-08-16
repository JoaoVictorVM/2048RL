package game

import (
	"math/rand"
	"time"
)

const initialTiles = 2

type Game struct {
	board Board
	score int
	rng   *rand.Rand
	won   bool

	presetBoard bool
}

type Option func(*Game)

func WithRNG(rng *rand.Rand) Option {
	return func(g *Game) {
		if rng != nil {
			g.rng = rng
		}
	}
}

func WithSeed(seed int64) Option {
	return WithRNG(rand.New(rand.NewSource(seed)))
}

func WithBoard(b Board) Option {
	return func(g *Game) {
		g.board = b
		g.presetBoard = true
	}
}

func WithScore(score int) Option {
	return func(g *Game) {
		g.score = score
	}
}

func NewGame(opts ...Option) *Game {
	g := &Game{}
	for _, opt := range opts {
		opt(g)
	}
	if g.rng == nil {
		g.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	if g.presetBoard {
		g.won = maxTile(g.board) >= WinningTile
		return g
	}
	for i := 0; i < initialTiles; i++ {
		spawnTile(&g.board, g.rng)
	}
	return g
}

func (g *Game) ApplyMove(dir Direction) MoveResult {
	newBoard, gained, moved := applyMoveToBoard(g.board, dir)
	if !moved {
		return MoveResult{
			Moved:    false,
			GameOver: g.IsGameOver(),
			Won:      g.won,
		}
	}

	g.board = newBoard
	g.score += gained

	result := MoveResult{Moved: true, ScoreGained: gained}
	if cell, value, ok := spawnTile(&g.board, g.rng); ok {
		result.Spawned = true
		result.SpawnedCell = cell
		result.SpawnedValue = value
	}

	if !g.won && maxTile(g.board) >= WinningTile {
		g.won = true
	}
	result.Won = g.won
	result.GameOver = g.IsGameOver()
	return result
}

func (g *Game) ValidMoves() []Direction {
	moves := make([]Direction, 0, len(AllDirections))
	for _, d := range AllDirections {
		if _, _, moved := applyMoveToBoard(g.board, d); moved {
			moves = append(moves, d)
		}
	}
	return moves
}

func (g *Game) Board() Board { return g.board }

func (g *Game) Score() int { return g.score }

func (g *Game) MaxTile() int { return maxTile(g.board) }

func (g *Game) IsGameOver() bool { return !canMove(g.board) }

func (g *Game) IsWon() bool { return g.won }
