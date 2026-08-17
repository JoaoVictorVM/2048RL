package agent

import (
	"errors"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

func trainedTestNetwork(t *testing.T, seed int64) *Network {
	t.Helper()
	n := newTestNetwork(t)

	rng := rand.New(rand.NewSource(seed))
	for _, table := range n.weights {
		for i := range table {
			table[i] = rng.Float32()*20 - 10
		}
	}
	return n
}

func evaluationBoards() []game.Board {
	return []game.Board{
		fixtureBoard(),
		{
			{2, 2, 4, 4},
			{8, 0, 0, 2},
			{0, 4, 2, 0},
			{2, 0, 0, 4},
		},
		{
			{0, 0, 0, 0},
			{0, 2, 0, 0},
			{0, 0, 4, 0},
			{0, 0, 0, 0},
		},
	}
}

func TestSerialize_RoundTripProducesIdenticalOutputs(t *testing.T) {
	saved := trainedTestNetwork(t, 42)
	path := filepath.Join(t.TempDir(), "weights", "run-a", "weights_ep1000.bin")

	if err := saved.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadNetwork(testConfig(), path)
	if err != nil {
		t.Fatalf("LoadNetwork: %v", err)
	}

	for i, board := range evaluationBoards() {
		if got, want := loaded.Evaluate(board), saved.Evaluate(board); math.Abs(got-want) > tolerance {
			t.Errorf("tabuleiro %d: Evaluate %v, esperado %v", i, got, want)
		}

		g := game.NewGame(game.WithBoard(board), game.WithRNG(rand.New(rand.NewSource(9))))
		wantDir, wantValue, wantOK := saved.SelectMove(g)
		gotDir, gotValue, gotOK := loaded.SelectMove(g)

		if gotOK != wantOK || gotDir != wantDir || math.Abs(gotValue-wantValue) > tolerance {
			t.Errorf("tabuleiro %d: SelectMove (%s, %v, %v), esperado (%s, %v, %v)",
				i, gotDir, gotValue, gotOK, wantDir, wantValue, wantOK)
		}
	}
}

func TestLoad_CorruptedFileFailsExplicitly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weights_ep1.bin")

	valid := trainedTestNetwork(t, 7)
	if err := valid.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(path, content[:len(content)/2], 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	target := newTestNetwork(t)
	err = target.Load(path)
	if err == nil {
		t.Fatal("esperado erro ao carregar checkpoint corrompido")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("o erro deveria identificar o arquivo, obtido: %v", err)
	}
	for i, table := range target.weights {
		for j, w := range table {
			if w != 0 {
				t.Fatalf("uma carga malsucedida não deveria alterar os pesos (tabela %d, entrada %d = %v)", i, j, w)
			}
		}
	}
}

func TestLoad_MissingFileFailsExplicitly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ausente.bin")

	err := newTestNetwork(t).Load(path)
	if err == nil {
		t.Fatal("esperado erro ao carregar checkpoint inexistente")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("o erro deveria identificar o arquivo, obtido: %v", err)
	}
}

func TestLoad_TupleConfigMismatchRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weights_ep1.bin")

	if err := trainedTestNetwork(t, 3).Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	other := testConfig()
	other.Tuples[0] = Tuple{Cell(3, 3), Cell(3, 2), Cell(2, 3)}

	if _, err := LoadNetwork(other, path); !errors.Is(err, ErrConfigMismatch) {
		t.Fatalf("esperado ErrConfigMismatch, obtido: %v", err)
	}
}

func TestLoad_MaxExponentMismatchRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weights_ep1.bin")

	if err := trainedTestNetwork(t, 3).Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	other := testConfig()
	other.MaxExponent = 4

	if _, err := LoadNetwork(other, path); !errors.Is(err, ErrConfigMismatch) {
		t.Fatalf("esperado ErrConfigMismatch, obtido: %v", err)
	}
}

func TestSave_InterruptedWriteNeverCorruptsPreviousCheckpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "weights_ep1000.bin")

	original := trainedTestNetwork(t, 11)
	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	failure := errors.New("disco cheio")
	if err := writeAtomic(path, func(w io.Writer) error {
		if _, err := w.Write([]byte("lixo parcial")); err != nil {
			return err
		}
		return failure
	}); !errors.Is(err, failure) {
		t.Fatalf("esperada a falha de escrita propagada, obtido: %v", err)
	}

	reloaded, err := LoadNetwork(testConfig(), path)
	if err != nil {
		t.Fatalf("o checkpoint anterior deveria continuar carregável: %v", err)
	}
	for i, board := range evaluationBoards() {
		if got, want := reloaded.Evaluate(board), original.Evaluate(board); math.Abs(got-want) > tolerance {
			t.Errorf("tabuleiro %d: Evaluate %v após a falha, esperado %v", i, got, want)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != filepath.Base(path) {
			t.Errorf("arquivo residual após a falha de escrita: %s", entry.Name())
		}
	}
}

func TestSave_CreatesMissingRunDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weights", "run-20260816-153000", "weights_ep2000.bin")

	if err := newTestNetwork(t).Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("checkpoint não foi criado: %v", err)
	}
}
