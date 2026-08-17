package agent

import (
	"fmt"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

type Network struct {
	cfg     Config
	weights [][]float32
}

type MoveEval struct {
	Direction  game.Direction
	Afterstate game.Board
	Reward     int
	Value      float64
}

func New(cfg Config) (*Network, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	weights := make([][]float32, len(cfg.Tuples))
	for i, tuple := range cfg.Tuples {
		weights[i] = make([]float32, cfg.TableSize(tuple))
	}
	return &Network{cfg: cfg, weights: weights}, nil
}

func NewDefault() *Network {
	n, err := New(DefaultConfig())
	if err != nil {
		panic(fmt.Sprintf("agent: configuração padrão inválida: %v", err))
	}
	return n
}

func (n *Network) Config() Config { return n.cfg }

func (n *Network) FeatureCount() int { return len(n.cfg.Tuples) * SymmetryCount }

func (n *Network) Evaluate(b game.Board) float64 {
	boards := symmetries(b)

	sum := 0.0
	for t, tuple := range n.cfg.Tuples {
		for _, board := range boards {
			sum += float64(n.weights[t][n.cfg.index(board, tuple)])
		}
	}
	return sum
}

type feature struct {
	table int
	index int
}

// A delta é distribuída entre as entradas ativas de forma que Evaluate varie
// exatamente delta. Entradas repetidas (simetrias que caem no mesmo índice)
// contam ao quadrado no divisor, já que Evaluate também as soma repetidas vezes.
func (n *Network) Update(b game.Board, delta float64) {
	features := n.activeFeatures(b)

	denominator := 0
	for _, f := range features {
		for _, other := range features {
			if f == other {
				denominator++
			}
		}
	}

	share := float32(delta / float64(denominator))
	for _, f := range features {
		n.weights[f.table][f.index] += share
	}
}

func (n *Network) activeFeatures(b game.Board) []feature {
	boards := symmetries(b)

	features := make([]feature, 0, n.FeatureCount())
	for t, tuple := range n.cfg.Tuples {
		for _, board := range boards {
			features = append(features, feature{table: t, index: n.cfg.index(board, tuple)})
		}
	}
	return features
}

func (n *Network) EvaluateMoves(g *game.Game) []MoveEval {
	board := g.Board()
	valid := g.ValidMoves()

	evals := make([]MoveEval, 0, len(valid))
	for _, dir := range valid {
		afterstate, reward, moved := game.Afterstate(board, dir)
		if !moved {
			continue
		}
		evals = append(evals, MoveEval{
			Direction:  dir,
			Afterstate: afterstate,
			Reward:     reward,
			Value:      float64(reward) + n.Evaluate(afterstate),
		})
	}
	return evals
}

func (n *Network) SelectMove(g *game.Game) (game.Direction, float64, bool) {
	evals := n.EvaluateMoves(g)
	if len(evals) == 0 {
		return 0, 0, false
	}

	best := evals[0]
	for _, eval := range evals[1:] {
		if eval.Value > best.Value {
			best = eval
		}
	}
	return best.Direction, best.Value, true
}
