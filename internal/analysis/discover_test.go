package analysis

import (
	"testing"
	"time"

	"github.com/officialasishkumar/emberlens/internal/platform"
)

func TestBuildDiscoverUntriagedDataset(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	issues := []platform.Issue{
		{
			Number:    10,
			Title:     "Unanswered bug",
			State:     "open",
			Comments:  2,
			CreatedAt: now.Add(-12 * 24 * time.Hour),
			UpdatedAt: now.Add(-2 * 24 * time.Hour),
			User:      platform.User{Login: "alice"},
		},
	}

	result := BuildDiscoverUntriagedDataset(issues, nil, now, 7*24*time.Hour, DiscoverSortAge)
	if len(result.Records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(result.Records))
	}
	if got := result.Records[0]["issue"]; got != "#10" {
		t.Fatalf("issue = %v, want %q", got, "#10")
	}
	if got := result.Records[0]["age"]; got != "12.0d" {
		t.Fatalf("age = %v, want %q", got, "12.0d")
	}
}
