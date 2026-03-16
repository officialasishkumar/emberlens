package analysis

import (
	"testing"
	"time"

	"github.com/officialasishkumar/emberlens/internal/platform"
)

func TestBuildIssueResponseDataset(t *testing.T) {
	created := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	responseAt := created.Add(6 * time.Hour)
	issues := []platform.Issue{
		{
			Number:    42,
			Title:     "Need help",
			State:     "open",
			Comments:  1,
			CreatedAt: created,
			UpdatedAt: responseAt,
			User:      platform.User{Login: "alice"},
		},
	}
	comments := map[int][]platform.IssueComment{
		42: {
			{
				User:              platform.User{Login: "maintainer"},
				AuthorAssociation: "MEMBER",
				CreatedAt:         responseAt,
			},
		},
	}

	result := BuildIssueResponseDataset(issues, comments, created.Add(-time.Hour), "hours")
	if len(result.Records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(result.Records))
	}
	if got := result.Records[0]["response"]; got != "6.0h" {
		t.Fatalf("response = %v, want %q", got, "6.0h")
	}
	if got := result.Records[0]["responder"]; got != "maintainer" {
		t.Fatalf("responder = %v, want %q", got, "maintainer")
	}
}

func TestBuildIssueAbandonedDataset(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	created := now.Add(-45 * 24 * time.Hour)
	updated := now.Add(-40 * 24 * time.Hour)
	issues := []platform.Issue{
		{
			Number:    7,
			Title:     "Stale issue",
			State:     "open",
			Comments:  0,
			CreatedAt: created,
			UpdatedAt: updated,
			User:      platform.User{Login: "reporter"},
		},
	}

	result := BuildIssueAbandonedDataset(issues, nil, now, 30*24*time.Hour)
	if len(result.Records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(result.Records))
	}
	if got := result.Records[0]["inactive"]; got != "40.0d" {
		t.Fatalf("inactive = %v, want %q", got, "40.0d")
	}
}

func TestBuildIssueCountsDatasetSkipsPullRequests(t *testing.T) {
	windowStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	closedAt := windowStart.Add(48 * time.Hour)

	issues := []platform.Issue{
		{
			Number:    1,
			State:     "open",
			CreatedAt: windowStart.Add(24 * time.Hour),
		},
		{
			Number:    2,
			State:     "closed",
			CreatedAt: windowStart.Add(12 * time.Hour),
			ClosedAt:  &closedAt,
		},
		{
			Number:      3,
			State:       "open",
			CreatedAt:   windowStart.Add(12 * time.Hour),
			PullRequest: struct{}{},
		},
	}

	result := BuildIssueCountsDataset(issues, windowStart)
	if len(result.Records) != 4 {
		t.Fatalf("len(records) = %d, want 4", len(result.Records))
	}
	if got := result.Records[0]["value"]; got != 1 {
		t.Fatalf("open issues = %v, want %d", got, 1)
	}
	if got := result.Records[1]["value"]; got != 1 {
		t.Fatalf("closed issues = %v, want %d", got, 1)
	}
	if got := result.Records[2]["value"]; got != 2 {
		t.Fatalf("new issues in window = %v, want %d", got, 2)
	}
}
