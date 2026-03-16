package app

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/officialasishkumar/emberlens/internal/platform"
)

type fakePlatformClient struct {
	issues       []platform.Issue
	issueCalls   []platform.IssueListOptions
	commentCalls []struct {
		number   int
		maxPages int
	}
}

func (f *fakePlatformClient) ProfileBaseURL() string { return "https://example.test" }

func (f *fakePlatformClient) GetRepo(context.Context, string, string) (platform.Repo, error) {
	return platform.Repo{}, nil
}

func (f *fakePlatformClient) ListContributors(context.Context, string, string, int) ([]platform.Contributor, error) {
	return nil, nil
}

func (f *fakePlatformClient) ListPullRequests(context.Context, string, string, int) ([]platform.PullRequest, error) {
	return nil, nil
}

func (f *fakePlatformClient) ListIssues(_ context.Context, _, _ string, opts platform.IssueListOptions) ([]platform.Issue, error) {
	f.issueCalls = append(f.issueCalls, opts)
	return append([]platform.Issue(nil), f.issues...), nil
}

func (f *fakePlatformClient) ListIssueComments(_ context.Context, _, _ string, number int, maxPages int) ([]platform.IssueComment, error) {
	f.commentCalls = append(f.commentCalls, struct {
		number   int
		maxPages int
	}{number: number, maxPages: maxPages})
	return nil, nil
}

func (f *fakePlatformClient) ListOrgMembers(context.Context, string) ([]platform.User, error) {
	return nil, nil
}

func (f *fakePlatformClient) ListCommitsSince(context.Context, string, string, time.Time, int) ([]platform.Commit, error) {
	return nil, nil
}

func (f *fakePlatformClient) GetProfile(context.Context, string) (platform.Profile, error) {
	return platform.Profile{}, nil
}

func newTestRunner(client platform.Client, now time.Time, stdout, stderr *bytes.Buffer) *Runner {
	runner := NewRunner(stdout, stderr)
	runner.Now = func() time.Time { return now }
	runner.newGitHubClient = func(string) platform.Client { return client }
	runner.newGitLabClient = func(string, string) platform.Client { return client }
	return runner
}

func TestRunnerHelpShowsUnifiedIssuesCommand(t *testing.T) {
	help := NewRunner(io.Discard, io.Discard).helpText()

	if !strings.Contains(help, "issues") {
		t.Fatalf("help text missing unified issues command:\n%s", help)
	}

	for _, legacy := range []string{
		"issues-new",
		"issues-active",
		"issues-closed",
		"issue-backlog",
		"issue-age",
		"issue-resolution",
		"issue-response",
		"issue-participants",
		"issue-abandoned",
		"issue-counts",
	} {
		if strings.Contains(help, legacy) {
			t.Fatalf("help text unexpectedly contains legacy command %q:\n%s", legacy, help)
		}
	}
}

func TestRunnerIssuesDefaultsToCountsView(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	closedAt := now.Add(-2 * time.Hour)
	client := &fakePlatformClient{
		issues: []platform.Issue{
			{Number: 1, State: "open", CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-time.Hour)},
			{Number: 2, State: "closed", CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: closedAt, ClosedAt: &closedAt},
		},
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runner := newTestRunner(client, now, stdout, stderr)

	code := runner.Run([]string{"issues", "-repo", "owner/repo", "-output", "json", "-no-report"}, "", "")
	if code != 0 {
		t.Fatalf("Run() = %d, stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if len(client.issueCalls) != 1 {
		t.Fatalf("len(issueCalls) = %d, want 1", len(client.issueCalls))
	}
	if got := client.issueCalls[0]; got.State != "all" || got.Sort != "updated" || got.Direction != "desc" {
		t.Fatalf("issue query = %+v, want state=all sort=updated direction=desc", got)
	}
	if !strings.Contains(stdout.String(), `"title": "Issue counts"`) {
		t.Fatalf("stdout = %s, want Issue counts dataset", stdout.String())
	}
}

func TestRunnerIssuesBacklogViewDispatchesCorrectly(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	client := &fakePlatformClient{
		issues: []platform.Issue{
			{
				Number:    7,
				Title:     "Old open issue",
				State:     "open",
				CreatedAt: now.Add(-45 * 24 * time.Hour),
				UpdatedAt: now.Add(-10 * 24 * time.Hour),
				User:      platform.User{Login: "alice"},
			},
		},
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runner := newTestRunner(client, now, stdout, stderr)

	code := runner.Run([]string{"issues", "-repo", "owner/repo", "-view", "backlog", "-sort", "updated", "-output", "json", "-no-report"}, "", "")
	if code != 0 {
		t.Fatalf("Run() = %d, stderr=%s", code, stderr.String())
	}
	if len(client.issueCalls) != 1 {
		t.Fatalf("len(issueCalls) = %d, want 1", len(client.issueCalls))
	}
	if got := client.issueCalls[0]; got.State != "open" || got.Sort != "created" || got.Direction != "asc" {
		t.Fatalf("issue query = %+v, want state=open sort=created direction=asc", got)
	}
	if !strings.Contains(stdout.String(), `"title": "Issue backlog"`) {
		t.Fatalf("stdout = %s, want Issue backlog dataset", stdout.String())
	}
}

func TestRunnerLegacyIssueAliasStillWorks(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	client := &fakePlatformClient{
		issues: []platform.Issue{
			{Number: 1, State: "open", CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-time.Hour)},
		},
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runner := newTestRunner(client, now, stdout, stderr)

	code := runner.Run([]string{"issue-counts", "-repo", "owner/repo", "-output", "json", "-no-report"}, "", "")
	if code != 0 {
		t.Fatalf("Run() = %d, stderr=%s", code, stderr.String())
	}
	if len(client.issueCalls) != 1 {
		t.Fatalf("len(issueCalls) = %d, want 1", len(client.issueCalls))
	}
	if !strings.Contains(stdout.String(), `"title": "Issue counts"`) {
		t.Fatalf("stdout = %s, want Issue counts dataset", stdout.String())
	}
}

func TestRunnerIssuesRejectsUnsupportedSort(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	client := &fakePlatformClient{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runner := newTestRunner(client, now, stdout, stderr)

	code := runner.Run([]string{"issues", "-repo", "owner/repo", "-sort", "age", "-no-report"}, "", "")
	if code != 1 {
		t.Fatalf("Run() = %d, want 1", code)
	}
	if len(client.issueCalls) != 0 {
		t.Fatalf("len(issueCalls) = %d, want 0", len(client.issueCalls))
	}
	if !strings.Contains(stderr.String(), "-sort is only supported") {
		t.Fatalf("stderr = %q, want unsupported -sort error", stderr.String())
	}
}
