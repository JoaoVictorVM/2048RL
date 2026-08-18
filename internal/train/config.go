package train

import "fmt"

const (
	DefaultLearningRate       = 0.0025
	DefaultCheckpointInterval = 1000
	DefaultLogInterval        = 100
	DefaultDataDir            = "./data"
)

type Config struct {
	Episodes           int
	LearningRate       float64
	CheckpointInterval int
	LogInterval        int
	RunID              string
	DataDir            string
	Seed               int64
}

func (c Config) Validate() error {
	if c.Episodes <= 0 {
		return fmt.Errorf("train: --episodes deve ser maior que zero, recebido %d", c.Episodes)
	}
	if c.LearningRate <= 0 {
		return fmt.Errorf("train: --learning-rate deve ser maior que zero, recebido %v", c.LearningRate)
	}
	if c.CheckpointInterval <= 0 {
		return fmt.Errorf("train: --checkpoint-interval deve ser maior que zero, recebido %d", c.CheckpointInterval)
	}
	if c.LogInterval <= 0 {
		return fmt.Errorf("train: --log-interval deve ser maior que zero, recebido %d", c.LogInterval)
	}
	if c.DataDir == "" {
		return fmt.Errorf("train: --data-dir não pode ser vazio")
	}
	return nil
}
