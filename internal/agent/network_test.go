package agent

import (
	"math"
	"math/rand"
	"testing"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

const tolerance = 1e-3

func testConfig() Config {
	return Config{
		MaxExponent: 3,
		Tuples: []Tuple{
			{Cell(0, 0), Cell(0, 1), Cell(1, 0)},
			{Cell(0, 1), Cell(0, 2), Cell(1, 1)},
			{Cell(1, 1), Cell(2, 1), Cell(2, 2)},
		},
	}
}

func newTestNetwork(t *testing.T) *Network {
	t.Helper()
	n, err := New(testConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return n
}

func fixtureBoard() game.Board {
	return game.Board{
		{2, 4, 2, 0},
		{4, 2, 0, 0},
		{0, 2, 4, 0},
		{0, 0, 0, 0},
	}
}

func TestNetwork_EvaluateSumsAcrossTuplesAndSymmetries(t *testing.T) {
	n := newTestNetwork(t)

	if got := n.Evaluate(fixtureBoard()); got != 0 {
		t.Fatalf("rede zerada deveria avaliar 0, obtido %v", got)
	}

	for _, table := range n.weights {
		for i := range table {
			table[i] = 1
		}
	}

	want := float64(len(n.Config().Tuples) * SymmetryCount)
	if got := n.Evaluate(fixtureBoard()); math.Abs(got-want) > tolerance {
		t.Errorf("esperado %v (tuplas x simetrias), obtido %v", want, got)
	}
	if got := n.FeatureCount(); got != len(testConfig().Tuples)*SymmetryCount {
		t.Errorf("FeatureCount inesperado: %d", got)
	}
}

func TestNetwork_UpdateChangesEvaluateByExpectedDelta(t *testing.T) {
	n := newTestNetwork(t)
	board := fixtureBoard()

	for _, delta := range []float64{24, -12, 0.5} {
		before := n.Evaluate(board)
		n.Update(board, delta)
		after := n.Evaluate(board)

		if got := after - before; math.Abs(got-delta) > tolerance {
			t.Errorf("delta %v: Evaluate variou %v", delta, got)
		}
	}
}

func TestNetwork_UpdateSharesWeightsAcrossSymmetricBoards(t *testing.T) {
	n := newTestNetwork(t)
	board := fixtureBoard()

	n.Update(board, 48)

	for i, symmetric := range symmetries(board) {
		if got, want := n.Evaluate(symmetric), n.Evaluate(board); math.Abs(got-want) > tolerance {
			t.Errorf("simetria %d avaliou %v, esperado %v", i, got, want)
		}
	}
}

func TestNetwork_UpdateContractIsStableForTraining(t *testing.T) {
	n := newTestNetwork(t)

	var update func(game.Board, float64) = n.Update
	var evaluate func(game.Board) float64 = n.Evaluate

	rng := rand.New(rand.NewSource(7))
	board := fixtureBoard()

	for i := 0; i < 20; i++ {
		delta := rng.Float64()*20 - 10
		before := evaluate(board)
		update(board, delta)

		if got := evaluate(board) - before; math.Abs(got-delta) > tolerance {
			t.Fatalf("iteração %d: esperado que Evaluate variasse %v, variou %v", i, delta, got)
		}
	}
}

func TestSelectMove_PicksHighestValuedAfterstate(t *testing.T) {
	n := newTestNetwork(t)
	g := game.NewGame(game.WithBoard(fixtureBoard()), game.WithRNG(rand.New(rand.NewSource(3))))

	evals := n.EvaluateMoves(g)
	if len(evals) < 2 {
		t.Fatalf("fixture precisa de ao menos dois movimentos válidos, tem %d", len(evals))
	}

	target := evals[len(evals)-1]
	n.Update(target.Afterstate, 1e6)

	dir, value, ok := n.SelectMove(g)
	if !ok {
		t.Fatal("esperado um movimento selecionado")
	}
	if dir != target.Direction {
		t.Errorf("esperado o movimento %s, obtido %s", target.Direction, dir)
	}
	if value <= 0 {
		t.Errorf("esperado valor positivo para o afterstate reforçado, obtido %v", value)
	}
}

func TestEvaluateMoves_CoversEveryValidMove(t *testing.T) {
	n := newTestNetwork(t)
	g := game.NewGame(game.WithBoard(fixtureBoard()), game.WithRNG(rand.New(rand.NewSource(3))))

	evals := n.EvaluateMoves(g)
	valid := g.ValidMoves()

	if len(evals) != len(valid) {
		t.Fatalf("esperadas %d avaliações, obtidas %d", len(valid), len(evals))
	}
	for i, eval := range evals {
		if eval.Direction != valid[i] {
			t.Errorf("posição %d: esperado %s, obtido %s", i, valid[i], eval.Direction)
		}
		afterstate, reward, moved := game.Afterstate(g.Board(), eval.Direction)
		if !moved || afterstate != eval.Afterstate || reward != eval.Reward {
			t.Errorf("%s: avaliação não corresponde ao afterstate do motor", eval.Direction)
		}
		if want := float64(reward) + n.Evaluate(afterstate); math.Abs(eval.Value-want) > tolerance {
			t.Errorf("%s: valor %v, esperado %v", eval.Direction, eval.Value, want)
		}
	}
}

func TestSelectMove_NoValidMoves(t *testing.T) {
	n := newTestNetwork(t)
	dead := game.Board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}
	g := game.NewGame(game.WithBoard(dead), game.WithRNG(rand.New(rand.NewSource(1))))

	if !g.IsGameOver() {
		t.Fatal("fixture deveria estar em game over")
	}
	if _, _, ok := n.SelectMove(g); ok {
		t.Error("esperado ok=false quando não há movimentos válidos")
	}
	if evals := n.EvaluateMoves(g); len(evals) != 0 {
		t.Errorf("esperada nenhuma avaliação, obtidas %d", len(evals))
	}
}

func TestNew_RejectsInvalidConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Error("esperado erro para configuração vazia")
	}
}

func TestNewDefault_AllocatesOneTablePerTuple(t *testing.T) {
	n := NewDefault()
	cfg := DefaultConfig()

	if len(n.weights) != len(cfg.Tuples) {
		t.Fatalf("esperadas %d tabelas, obtidas %d", len(cfg.Tuples), len(n.weights))
	}
	for i, tuple := range cfg.Tuples {
		if got, want := len(n.weights[i]), cfg.TableSize(tuple); got != want {
			t.Errorf("tabela %d: %d entradas, esperadas %d", i, got, want)
		}
	}
}
