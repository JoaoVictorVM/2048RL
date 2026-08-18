package train

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/metrics"
	"github.com/JoaoVictorVM/2048RL/internal/web"
)

const (
	runIDLayout    = "20060102-150405"
	maxRunIDSuffix = 1000
)

func GenerateRunID(now time.Time) string {
	return "run-" + now.UTC().Format(runIDLayout)
}

// Um ID gerado automaticamente ganha sufixo numérico quando colide (dois treinos
// iniciados no mesmo segundo); um ID escolhido pelo usuário aborta, para não
// renomear em silêncio o que ele nomeou de propósito.
func ResolveRunID(dataDir, requested string, now time.Time) (string, error) {
	if requested != "" {
		if RunExists(dataDir, requested) {
			return "", fmt.Errorf("train: o run %q já existe em %s", requested, dataDir)
		}
		return requested, nil
	}

	base := GenerateRunID(now)
	if !RunExists(dataDir, base) {
		return base, nil
	}
	for suffix := 2; suffix < maxRunIDSuffix; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if !RunExists(dataDir, candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("train: não foi possível gerar um run id livre a partir de %q", base)
}

func RunExists(dataDir, runID string) bool {
	for _, dir := range []string{web.WeightsDirName, metrics.DirName} {
		if _, err := os.Stat(filepath.Join(dataDir, dir, runID)); err == nil {
			return true
		}
	}
	return false
}

func WeightsDir(dataDir, runID string) string {
	return filepath.Join(dataDir, web.WeightsDirName, runID)
}

func CheckpointPath(dataDir, runID string, episode int) string {
	return filepath.Join(WeightsDir(dataDir, runID), web.CheckpointFilename(episode))
}
