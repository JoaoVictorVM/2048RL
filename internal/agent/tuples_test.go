package agent

import (
	"testing"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

func TestTupleIndex_KnownBoardProducesKnownIndex(t *testing.T) {
	cfg := DefaultConfig()
	board := game.Board{
		{2, 4, 8, 0},
		{16, 32, 64, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}

	const want = 1*1 + 2*16 + 3*256 + 4*4096 + 5*65536 + 6*1048576

	if got := cfg.index(board, cfg.Tuples[0]); got != want {
		t.Errorf("índice esperado %d, obtido %d", want, got)
	}
}

func TestTupleIndex_HandlesEmptyAndCappedCells(t *testing.T) {
	cfg := DefaultConfig()

	if got := cfg.exponent(0); got != 0 {
		t.Errorf("célula vazia deveria codificar como 0, obtido %d", got)
	}
	if got := cfg.exponent(2); got != 1 {
		t.Errorf("tile 2 deveria codificar como 1, obtido %d", got)
	}
	if got := cfg.exponent(32768); got != DefaultMaxExponent {
		t.Errorf("tile 32768 deveria codificar como %d, obtido %d", DefaultMaxExponent, got)
	}
	if got := cfg.exponent(65536); got != DefaultMaxExponent {
		t.Errorf("tile acima do teto deveria saturar em %d, obtido %d", DefaultMaxExponent, got)
	}

	empty := game.Board{}
	if got := cfg.index(empty, cfg.Tuples[0]); got != 0 {
		t.Errorf("tabuleiro vazio deveria produzir índice 0, obtido %d", got)
	}

	capped := game.Board{
		{32768, 32768, 32768, 0},
		{32768, 32768, 32768, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	if got, want := cfg.index(capped, cfg.Tuples[0]), cfg.TableSize(cfg.Tuples[0])-1; got != want {
		t.Errorf("tabuleiro saturado deveria produzir o último índice %d, obtido %d", want, got)
	}
}

func TestConfig_TableSizeMatchesBasePowerOfTupleLength(t *testing.T) {
	cfg := DefaultConfig()
	if got, want := cfg.Base(), 16; got != want {
		t.Fatalf("base esperada %d, obtida %d", want, got)
	}
	for i, tuple := range cfg.Tuples {
		if len(tuple) != 6 {
			t.Errorf("tupla %d deveria ter 6 células, tem %d", i, len(tuple))
		}
		if got, want := cfg.TableSize(tuple), 16*16*16*16*16*16; got != want {
			t.Errorf("tupla %d: tamanho esperado %d, obtido %d", i, want, got)
		}
	}
}

func TestConfig_Validate(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("configuração padrão deveria ser válida: %v", err)
	}

	cases := map[string]Config{
		"sem tuplas":      {Tuples: nil, MaxExponent: 15},
		"expoente zero":   {Tuples: []Tuple{{Cell(0, 0)}}, MaxExponent: 0},
		"tupla vazia":     {Tuples: []Tuple{{}}, MaxExponent: 15},
		"fora do board":   {Tuples: []Tuple{{Cell(0, game.Size)}}, MaxExponent: 15},
		"célula negativa": {Tuples: []Tuple{{Cell(-1, 0)}}, MaxExponent: 15},
	}
	for name, cfg := range cases {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: esperado erro de validação", name)
		}
	}
}

func TestConfig_HashDistinguishesTupleDefinitions(t *testing.T) {
	base := DefaultConfig()
	if base.Hash() != DefaultConfig().Hash() {
		t.Error("o hash deveria ser estável para a mesma configuração")
	}

	changed := DefaultConfig()
	changed.Tuples[0] = Tuple{Cell(3, 3), Cell(3, 2), Cell(3, 1), Cell(2, 3), Cell(2, 2), Cell(2, 1)}
	if base.Hash() == changed.Hash() {
		t.Error("o hash deveria mudar quando as coordenadas das tuplas mudam")
	}
}
