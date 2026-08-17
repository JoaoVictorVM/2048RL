package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/bits"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

const DefaultMaxExponent = 15

type Tuple []game.Cell

type Config struct {
	Tuples      []Tuple
	MaxExponent int
}

func Cell(row, col int) game.Cell {
	return game.Cell{Row: row, Col: col}
}

func DefaultConfig() Config {
	return Config{
		MaxExponent: DefaultMaxExponent,
		Tuples: []Tuple{
			{Cell(0, 0), Cell(0, 1), Cell(0, 2), Cell(1, 0), Cell(1, 1), Cell(1, 2)},
			{Cell(0, 1), Cell(0, 2), Cell(0, 3), Cell(1, 1), Cell(1, 2), Cell(1, 3)},
			{Cell(0, 0), Cell(1, 0), Cell(2, 0), Cell(0, 1), Cell(1, 1), Cell(2, 1)},
			{Cell(0, 0), Cell(0, 1), Cell(0, 2), Cell(0, 3), Cell(1, 0), Cell(1, 1)},
		},
	}
}

func (c Config) Base() int { return c.MaxExponent + 1 }

func (c Config) TableSize(tuple Tuple) int {
	size := 1
	for range tuple {
		size *= c.Base()
	}
	return size
}

func (c Config) Validate() error {
	if len(c.Tuples) == 0 {
		return fmt.Errorf("agent: configuração sem tuplas")
	}
	if c.MaxExponent < 1 {
		return fmt.Errorf("agent: expoente máximo inválido: %d", c.MaxExponent)
	}
	for i, tuple := range c.Tuples {
		if len(tuple) == 0 {
			return fmt.Errorf("agent: tupla %d está vazia", i)
		}
		for _, cell := range tuple {
			if cell.Row < 0 || cell.Row >= game.Size || cell.Col < 0 || cell.Col >= game.Size {
				return fmt.Errorf("agent: tupla %d referencia célula fora do tabuleiro: %+v", i, cell)
			}
		}
	}
	return nil
}

func (c Config) Hash() string {
	h := sha256.New()
	for _, tuple := range c.Tuples {
		for _, cell := range tuple {
			fmt.Fprintf(h, "%d,%d;", cell.Row, cell.Col)
		}
		fmt.Fprint(h, "|")
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c Config) exponent(value int) int {
	if value < 2 {
		return 0
	}
	exp := bits.Len(uint(value)) - 1
	if exp > c.MaxExponent {
		return c.MaxExponent
	}
	return exp
}

func (c Config) index(b game.Board, tuple Tuple) int {
	index := 0
	weight := 1
	for _, cell := range tuple {
		index += c.exponent(b[cell.Row][cell.Col]) * weight
		weight *= c.Base()
	}
	return index
}
