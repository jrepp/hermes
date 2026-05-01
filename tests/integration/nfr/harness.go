//go:build integration && nfr
// +build integration,nfr

package nfr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	if cfg.Duration == 0 {
		cfg.Duration = 2 * time.Minute
	}
	if cfg.Rate == "" {
		cfg.Rate = "60/min"
	}
	if cfg.RestartInterval == 0 {
		cfg.RestartInterval = time.Minute
	}
	if cfg.ConvergenceDeadline == 0 {
		cfg.ConvergenceDeadline = 30 * time.Second
	}
	if cfg.Output == "" {
		cfg.Output = filepath.Join("tmp", "nfr", time.Now().UTC().Format("20060102T150405Z"))
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
	if !strings.HasSuffix(cfg.Rate, "/min") && !strings.HasSuffix(cfg.Rate, "/hour") {
		return fmt.Errorf("rate must use /min or /hour suffix, got %q", cfg.Rate)
	}
	return nil
}

func newResult(scenario string, cfg harnessConfig, startedAt time.Time, obs observations, passed bool) result {
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
