package agent

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const FormatVersion = 1

var ErrConfigMismatch = errors.New("agent: checkpoint incompatível com a configuração de tuplas atual")

type checkpoint struct {
	FormatVersion   int
	TupleConfigHash string
	MaxExponent     int
	Weights         [][]float32
}

func (n *Network) Save(path string) error {
	return writeAtomic(path, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(checkpoint{
			FormatVersion:   FormatVersion,
			TupleConfigHash: n.cfg.Hash(),
			MaxExponent:     n.cfg.MaxExponent,
			Weights:         n.weights,
		})
	})
}

func (n *Network) Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("agent: não foi possível abrir o checkpoint %s: %w", path, err)
	}
	defer file.Close()

	var loaded checkpoint
	if err := gob.NewDecoder(file).Decode(&loaded); err != nil {
		return fmt.Errorf("agent: checkpoint %s corrompido ou em formato inválido: %w", path, err)
	}

	if loaded.FormatVersion != FormatVersion {
		return fmt.Errorf("agent: checkpoint %s usa a versão de formato %d, esperada %d: %w",
			path, loaded.FormatVersion, FormatVersion, ErrConfigMismatch)
	}
	if loaded.TupleConfigHash != n.cfg.Hash() {
		return fmt.Errorf("agent: checkpoint %s foi salvo com outra configuração de tuplas: %w", path, ErrConfigMismatch)
	}
	if loaded.MaxExponent != n.cfg.MaxExponent {
		return fmt.Errorf("agent: checkpoint %s usa expoente máximo %d, esperado %d: %w",
			path, loaded.MaxExponent, n.cfg.MaxExponent, ErrConfigMismatch)
	}
	if len(loaded.Weights) != len(n.cfg.Tuples) {
		return fmt.Errorf("agent: checkpoint %s tem %d tabelas de pesos, esperadas %d: %w",
			path, len(loaded.Weights), len(n.cfg.Tuples), ErrConfigMismatch)
	}
	for i, tuple := range n.cfg.Tuples {
		if want := n.cfg.TableSize(tuple); len(loaded.Weights[i]) != want {
			return fmt.Errorf("agent: checkpoint %s tem tabela %d com %d entradas, esperadas %d: %w",
				path, i, len(loaded.Weights[i]), want, ErrConfigMismatch)
		}
	}

	n.weights = loaded.Weights
	return nil
}

func LoadNetwork(cfg Config, path string) (*Network, error) {
	n, err := New(cfg)
	if err != nil {
		return nil, err
	}
	if err := n.Load(path); err != nil {
		return nil, err
	}
	return n, nil
}

// Escreve em arquivo temporário e renomeia, para que uma escrita interrompida
// nunca corrompa o checkpoint válido anterior.
func writeAtomic(path string, write func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("agent: não foi possível criar o diretório %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp*")
	if err != nil {
		return fmt.Errorf("agent: não foi possível criar o arquivo temporário em %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if err := write(tmp); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("agent: falha ao escrever o checkpoint %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("agent: falha ao sincronizar o checkpoint %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("agent: falha ao fechar o checkpoint temporário de %s: %w", path, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("agent: falha ao publicar o checkpoint %s: %w", path, err)
	}
	return nil
}
