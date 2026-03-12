package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/analysis"
	"gopkg.in/yaml.v3"
)

const defaultReportDir = "emberlens-reports"

// Report captures the full output of a single emberlens run.
type Report struct {
	Version    string            `yaml:"version"`
	Name       string            `yaml:"name"`
	Command    string            `yaml:"command"`
	Repo       string            `yaml:"repo"`
	Subcommand string            `yaml:"subcommand"`
	Status     string            `yaml:"status"`
	Total      int               `yaml:"total"`
	CreatedAt  string            `yaml:"created_at"`
	TimeTaken  string            `yaml:"time_taken"`
	People     []analysis.Person `yaml:"people"`
}

// Save writes a report YAML file into the report directory, organized like:
//
//	<reportDir>/<subcommand>/run-<N>/report.yaml
func Save(reportDir string, subcommand string, repo string, command string, people []analysis.Person, elapsed time.Duration) (string, error) {
	if reportDir == "" {
		reportDir = defaultReportDir
	}

	subDir := filepath.Join(reportDir, subcommand)
	runIndex := nextRunIndex(subDir)
	runDir := filepath.Join(subDir, fmt.Sprintf("run-%d", runIndex))

	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create report directory: %w", err)
	}

	r := Report{
		Version:    "v1",
		Name:       fmt.Sprintf("%s-run-%d", subcommand, runIndex),
		Command:    command,
		Repo:       repo,
		Subcommand: subcommand,
		Status:     "success",
		Total:      len(people),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		TimeTaken:  elapsed.Round(time.Millisecond).String(),
		People:     people,
	}

	data, err := yaml.Marshal(&r)
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}

	reportPath := filepath.Join(runDir, "report.yaml")
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}

	return reportPath, nil
}

// nextRunIndex scans the subcommand directory for existing run-N folders
// and returns the next available index.
func nextRunIndex(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	maxIdx := -1
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "run-") {
			continue
		}
		idxStr := strings.TrimPrefix(name, "run-")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			continue
		}
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	return maxIdx + 1
}
