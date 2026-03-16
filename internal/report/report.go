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
	Version   string              `yaml:"version"`
	Name      string              `yaml:"name"`
	Command   string              `yaml:"command"`
	Repo      string              `yaml:"repo"`
	Status    string              `yaml:"status"`
	Total     int                 `yaml:"total"`
	CreatedAt string              `yaml:"created_at"`
	TimeTaken string              `yaml:"time_taken"`
	Result    analysis.Dataset    `yaml:"result"`
}

// Save writes a report YAML file into the report directory, organized like:
//
//	<reportDir>/test-run-<N>/report.yaml
func Save(reportDir string, repo string, command string, result analysis.Dataset, elapsed time.Duration) (string, error) {
	if reportDir == "" {
		reportDir = defaultReportDir
	}

	runIndex := nextRunIndex(reportDir)
	runDir := filepath.Join(reportDir, fmt.Sprintf("test-run-%d", runIndex))

	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create report directory: %w", err)
	}

	r := Report{
		Version:   "v2",
		Name:      fmt.Sprintf("test-run-%d", runIndex),
		Command:   command,
		Repo:      repo,
		Status:    "success",
		Total:     len(result.Records),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		TimeTaken: elapsed.Round(time.Millisecond).String(),
		Result:    result,
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

// nextRunIndex scans the report directory for existing test-run-N folders
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
		if !strings.HasPrefix(name, "test-run-") {
			continue
		}
		idxStr := strings.TrimPrefix(name, "test-run-")
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
