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

func TestBuildDiscoverNeedsMaintainerDataset(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	issues := []platform.Issue{
		{
			Number:    11,
			Title:     "Crowded issue",
			State:     "open",
			Comments:  4,
			CreatedAt: now.Add(-15 * 24 * time.Hour),
			UpdatedAt: now.Add(-6 * time.Hour),
			User:      platform.User{Login: "alice"},
		},
	}
	comments := map[int][]platform.IssueComment{
		11: {
			{User: platform.User{Login: "bob"}, CreatedAt: now.Add(-5 * time.Hour)},
			{User: platform.User{Login: "carol"}, CreatedAt: now.Add(-4 * time.Hour)},
			{User: platform.User{Login: "dave"}, CreatedAt: now.Add(-3 * time.Hour)},
		},
	}

	result := BuildDiscoverNeedsMaintainerDataset(issues, comments, now, 7*24*time.Hour, 3, 3, DiscoverSortDiscussion)
	if len(result.Records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(result.Records))
	}
	if got := result.Records[0]["participants"]; got != 4 {
		t.Fatalf("participants = %v, want %d", got, 4)
	}
	if got := result.Records[0]["comments"]; got != 4 {
		t.Fatalf("comments = %v, want %d", got, 4)
	}
}

func TestBuildDiscoverHotspotsDataset(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	issues := []platform.Issue{
		{
			Number:    12,
			Title:     "Hot issue",
			State:     "open",
			Comments:  5,
			CreatedAt: now.Add(-20 * 24 * time.Hour),
			UpdatedAt: now.Add(-4 * time.Hour),
			User:      platform.User{Login: "alice"},
		},
	}
	comments := map[int][]platform.IssueComment{
		12: {
			{User: platform.User{Login: "bob"}, CreatedAt: now.Add(-3 * time.Hour)},
			{User: platform.User{Login: "carol"}, CreatedAt: now.Add(-2 * time.Hour)},
			{User: platform.User{Login: "maintainer"}, AuthorAssociation: "MEMBER", CreatedAt: now.Add(-time.Hour)},
		},
	}

	result := BuildDiscoverHotspotsDataset(issues, comments, now, now.Add(-7*24*time.Hour), 5, 3, DiscoverSortHeat)
	if len(result.Records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(result.Records))
	}
	if got := result.Records[0]["maintainer"]; got != "maintainer" {
		t.Fatalf("maintainer = %v, want %q", got, "maintainer")
	}
	if got := result.Records[0]["participants"]; got != 4 {
		t.Fatalf("participants = %v, want %d", got, 4)
	}
}
