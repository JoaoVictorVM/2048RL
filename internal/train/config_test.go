package train

import "testing"

func validConfig() Config {
	return Config{
		Episodes:           10,
		LearningRate:       DefaultLearningRate,
		CheckpointInterval: DefaultCheckpointInterval,
		LogInterval:        DefaultLogInterval,
		DataDir:            "./data",
	}
}

func TestConfig_ValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("configuração válida rejeitada: %v", err)
	}
}

func TestConfig_ValidateRejectsInvalidValues(t *testing.T) {
	cases := map[string]func(*Config){
		"sem episódios":           func(c *Config) { c.Episodes = 0 },
		"episódios negativos":     func(c *Config) { c.Episodes = -1 },
		"learning rate zero":      func(c *Config) { c.LearningRate = 0 },
		"intervalo de checkpoint": func(c *Config) { c.CheckpointInterval = 0 },
		"intervalo de log":        func(c *Config) { c.LogInterval = 0 },
		"data dir vazio":          func(c *Config) { c.DataDir = "" },
	}

	for name, mutate := range cases {
		cfg := validConfig()
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: esperado erro de validação", name)
		}
	}
}
