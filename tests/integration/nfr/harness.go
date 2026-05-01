//go:build integration && nfr
// +build integration,nfr

package nfr

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type harnessConfig struct {
	Profile             string
	Rate                string
	Backend             string
	Output              string
	Duration            time.Duration
	RestartInterval     time.Duration
	ConvergenceDeadline time.Duration
	MaxItems            int64
}

type result struct {
	Scenario     string       `json:"scenario"`
	Profile      string       `json:"profile"`
	StartedAt    time.Time    `json:"startedAt"`
	FinishedAt   time.Time    `json:"finishedAt"`
	Command      string       `json:"command"`
	Environment  environment  `json:"environment"`
	Inputs       inputs       `json:"inputs"`
	Observations observations `json:"observations"`
	Passed       bool         `json:"passed"`
	FollowUps    []string     `json:"followUps"`
}

type environment struct {
	GitCommit string `json:"gitCommit"`
	GoVersion string `json:"goVersion"`
	OS        string `json:"os"`
	Backend   string `json:"backend"`
}

type inputs struct {
	DurationSeconds            int64  `json:"durationSeconds"`
	TargetRate                 string `json:"targetRate"`
	RestartIntervalSeconds     int64  `json:"restartIntervalSeconds"`
	ConvergenceDeadlineSeconds int64  `json:"convergenceDeadlineSeconds"`
	MaxItems                   int64  `json:"maxItems,omitempty"`
}

type observations struct {
	ItemsGenerated        int      `json:"itemsGenerated"`
	ItemsCompleted        int      `json:"itemsCompleted"`
	WorkerRestarts        int      `json:"workerRestarts"`
	MaxConvergenceSeconds int64    `json:"maxConvergenceSeconds"`
	MaxQueueDepth         int      `json:"maxQueueDepth"`
	UnexpectedDLQ         int      `json:"unexpectedDLQ"`
	Errors                []string `json:"errors"`
}

func applyDefaults(cfg harnessConfig) harnessConfig {
	if cfg.Profile == "" {
		cfg.Profile = envOrDefault("HERMES_NFR_PROFILE", "smoke")
	}
	if cfg.Backend == "" {
		cfg.Backend = envOrDefault("HERMES_NFR_BACKEND", "testcontainers")
	}
	if cfg.Duration == 0 || cfg.Rate == "" || cfg.RestartInterval == 0 || cfg.ConvergenceDeadline == 0 {
		cfg = applyProfileDefaults(cfg)
	}
	if cfg.Output == "" {
		cfg.Output = filepath.Join("tmp", "nfr", time.Now().UTC().Format("20060102T150405Z"))
	}
	if !filepath.IsAbs(cfg.Output) {
		cfg.Output = filepath.Join(repoRoot(), cfg.Output)
	}
	return cfg
}

func applyProfileDefaults(cfg harnessConfig) harnessConfig {
	switch cfg.Profile {
	case "release":
		if cfg.Duration == 0 {
			cfg.Duration = 10 * time.Minute
		}
		if cfg.Rate == "" {
			cfg.Rate = "1000/min"
		}
		if cfg.RestartInterval == 0 {
			cfg.RestartInterval = time.Minute
		}
	case "smoke":
		if cfg.Duration == 0 {
			cfg.Duration = 5 * time.Second
		}
		if cfg.Rate == "" {
			cfg.Rate = "12/min"
		}
		if cfg.RestartInterval == 0 {
			cfg.RestartInterval = time.Second
		}
	default:
		if cfg.Duration == 0 {
			cfg.Duration = 2 * time.Minute
		}
		if cfg.Rate == "" {
			cfg.Rate = "60/min"
		}
		if cfg.RestartInterval == 0 {
			cfg.RestartInterval = time.Minute
		}
	}
	if cfg.ConvergenceDeadline == 0 {
		cfg.ConvergenceDeadline = 30 * time.Second
	}
	return cfg
}

func validateConfig(cfg harnessConfig) error {
	switch cfg.Profile {
	case "smoke", "release":
	default:
		return fmt.Errorf("profile must be smoke or release, got %q", cfg.Profile)
	}
	switch cfg.Backend {
	case "local", "testcontainers", "external":
	default:
		return fmt.Errorf("backend must be local, testcontainers, or external, got %q", cfg.Backend)
	}
	if cfg.Duration <= 0 {
		return errors.New("duration must be positive")
	}
	if cfg.ConvergenceDeadline <= 0 {
		return errors.New("convergence deadline must be positive")
	}
	if _, err := intervalForRate(cfg.Rate); err != nil {
		return err
	}
	return nil
}

func intervalForRate(rate string) (time.Duration, error) {
	var period time.Duration
	var valueText string
	switch {
	case strings.HasSuffix(rate, "/min"):
		period = time.Minute
		valueText = strings.TrimSuffix(rate, "/min")
	case strings.HasSuffix(rate, "/hour"):
		period = time.Hour
		valueText = strings.TrimSuffix(rate, "/hour")
	default:
		return 0, fmt.Errorf("rate must use /min or /hour suffix, got %q", rate)
	}

	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("rate must have a positive numeric value, got %q", rate)
	}
	interval := time.Duration(math.Round(float64(period) / value))
	if interval <= 0 {
		return 0, fmt.Errorf("rate %q is too high for duration precision", rate)
	}
	return interval, nil
}

func newResult(scenario string, cfg harnessConfig, startedAt time.Time, obs observations, passed bool) result {
	if obs.Errors == nil {
		obs.Errors = []string{}
	}
	return result{
		Scenario:   scenario,
		Profile:    cfg.Profile,
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
		Command:    strings.Join(os.Args, " "),
		Environment: environment{
			GitCommit: envOrDefault("GIT_COMMIT", "unknown"),
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS + "/" + runtime.GOARCH,
			Backend:   cfg.Backend,
		},
		Inputs: inputs{
			DurationSeconds:            int64(cfg.Duration.Seconds()),
			TargetRate:                 cfg.Rate,
			RestartIntervalSeconds:     int64(cfg.RestartInterval.Seconds()),
			ConvergenceDeadlineSeconds: int64(cfg.ConvergenceDeadline.Seconds()),
			MaxItems:                   cfg.MaxItems,
		},
		Observations: obs,
		Passed:       passed,
		FollowUps:    []string{},
	}
}

func writeArtifacts(dir string, res result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create NFR output directory: %w", err)
	}

	jsonPath := filepath.Join(dir, "result.json")
	jsonFile, err := os.Create(jsonPath)
	if err != nil {
		return fmt.Errorf("create result.json: %w", err)
	}
	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(res)
	closeErr := jsonFile.Close()
	if encodeErr != nil {
		return fmt.Errorf("write result.json: %w", encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close result.json: %w", closeErr)
	}

	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(renderSummary(res)), 0o644); err != nil {
		return fmt.Errorf("write summary.md: %w", err)
	}
	return nil
}

func renderSummary(res result) string {
	status := "failed"
	if res.Passed {
		status = "passed"
	}
	return fmt.Sprintf("# NFR Result: %s\n\n"+
		"- Status: %s\n"+
		"- Profile: %s\n"+
		"- Command: `%s`\n"+
		"- Git commit: `%s`\n"+
		"- Environment: %s, %s\n"+
		"- Duration: %ds\n"+
		"- Target rate: %s\n"+
		"- Restart interval: %ds\n"+
		"- Convergence deadline: %ds\n"+
		"- Max items: %d\n"+
		"- Items generated: %d\n"+
		"- Items completed: %d\n"+
		"- Worker restarts: %d\n"+
		"- Max convergence: %ds\n"+
		"- Max queue depth: %d\n"+
		"- Unexpected DLQ: %d\n"+
		"- Errors: %s\n"+
		"- Follow-ups: %s\n",
		res.Scenario,
		status,
		res.Profile,
		res.Command,
		res.Environment.GitCommit,
		res.Environment.GoVersion,
		res.Environment.OS,
		res.Inputs.DurationSeconds,
		res.Inputs.TargetRate,
		res.Inputs.RestartIntervalSeconds,
		res.Inputs.ConvergenceDeadlineSeconds,
		res.Inputs.MaxItems,
		res.Observations.ItemsGenerated,
		res.Observations.ItemsCompleted,
		res.Observations.WorkerRestarts,
		res.Observations.MaxConvergenceSeconds,
		res.Observations.MaxQueueDepth,
		res.Observations.UnexpectedDLQ,
		listOrNone(res.Observations.Errors),
		listOrNone(res.FollowUps),
	)
}

func listOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
