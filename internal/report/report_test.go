package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/officialasishkumar/emberlens/internal/analysis"
)

func TestSaveUsesFlatTestRunLayout(t *testing.T) {
	dir := t.TempDir()
	result := analysis.Dataset{
		Title: "Issue counts",
		Records: []map[string]any{
			{"metric": "Open issues", "value": 12},
		},
	}

	path, err := Save(dir, "owner/repo", "emberlens issue-counts -repo=owner/repo", result, 1250*time.Millisecond)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	wantDir := filepath.Join(dir, "test-run-0")
	if filepath.Dir(path) != wantDir {
		t.Fatalf("report dir = %q, want %q", filepath.Dir(path), wantDir)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected report file at %q: %v", path, err)
	}
}
