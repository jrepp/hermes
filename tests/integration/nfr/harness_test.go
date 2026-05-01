//go:build integration && nfr
// +build integration,nfr

package nfr

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cfg harnessConfig

func init() {
	flag.StringVar(&cfg.Profile, "profile", "", "NFR profile: smoke or release")
	flag.DurationVar(&cfg.Duration, "duration", 0, "NFR scenario duration")
	flag.StringVar(&cfg.Rate, "rate", "", "NFR target rate, e.g. 1000/min or 1000/hour")
	flag.DurationVar(&cfg.RestartInterval, "restart-interval", 0, "NFR worker restart interval; 0 uses profile default")
	flag.DurationVar(&cfg.ConvergenceDeadline, "convergence-deadline", 0, "NFR convergence deadline")
	flag.Int64Var(&cfg.MaxItems, "max-items", 0, "Maximum generated items before stopping; 0 uses duration")
	flag.StringVar(&cfg.Output, "output", "", "NFR output directory")
	flag.StringVar(&cfg.Backend, "backend", "", "NFR backend: local, testcontainers, or external")
}

func TestNFR(t *testing.T) {
	t.Run("HarnessContract", func(t *testing.T) {
		config := applyDefaults(cfg)
		config.Output = t.TempDir()
		require.NoError(t, validateConfig(config))

		startedAt := time.Now().UTC()
		res := newResult("harness-contract", config, startedAt, observations{
			ItemsGenerated:        1,
			ItemsCompleted:        1,
			MaxConvergenceSeconds: 1,
		}, true)

		require.NoError(t, writeArtifacts(config.Output, res))

		resultBytes, err := os.ReadFile(filepath.Join(config.Output, "result.json"))
		require.NoError(t, err)
		var decoded result
		require.NoError(t, json.Unmarshal(resultBytes, &decoded))
		assert.Equal(t, "harness-contract", decoded.Scenario)
		assert.Equal(t, config.Profile, decoded.Profile)
		assert.True(t, decoded.Passed)

		summaryBytes, err := os.ReadFile(filepath.Join(config.Output, "summary.md"))
		require.NoError(t, err)
		assert.Contains(t, string(summaryBytes), "# NFR Result: harness-contract")
	})

	t.Run("SearchOutboxStress", testSearchOutboxStress)

	t.Run("OutputPath", func(t *testing.T) {
		config := applyDefaults(harnessConfig{Output: filepath.Join("tmp", "nfr", "contract")})
		assert.True(t, filepath.IsAbs(config.Output))
		assert.True(t, strings.HasSuffix(config.Output, filepath.Join("tmp", "nfr", "contract")))
	})
}
