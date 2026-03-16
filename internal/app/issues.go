package app

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/analysis"
	"github.com/officialasishkumar/emberlens/internal/platform"
)

type issuesNewCmd struct {
	since    time.Duration
	period   string
	maxPages int
}

func (c *issuesNewCmd) Name() string { return "issues-new" }
func (c *issuesNewCmd) Description() string {
	return "Show new issue volume over time"
}

func (c *issuesNewCmd) RegisterFlags(fs *flag.FlagSet) {
	registerIssueWindowFlags(fs, &c.since, &c.period, &c.maxPages)
}

func (c *issuesNewCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	period, err := normalizePeriod(c.period)
	if err != nil {
		return analysis.Dataset{}, err
	}
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "all",
		Sort:      "created",
		Direction: "desc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssuesNewDataset(issues, rc.Now.Add(-c.since), period), nil
}

type issuesActiveCmd struct {
	since    time.Duration
	period   string
	maxPages int
}

func (c *issuesActiveCmd) Name() string { return "issues-active" }
func (c *issuesActiveCmd) Description() string {
	return "Show active issues by latest activity time"
}

func (c *issuesActiveCmd) RegisterFlags(fs *flag.FlagSet) {
	registerIssueWindowFlags(fs, &c.since, &c.period, &c.maxPages)
}

func (c *issuesActiveCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	period, err := normalizePeriod(c.period)
	if err != nil {
		return analysis.Dataset{}, err
	}
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "all",
		Sort:      "updated",
		Direction: "desc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssuesActiveDataset(issues, rc.Now.Add(-c.since), period), nil
}

type issuesClosedCmd struct {
	since    time.Duration
	period   string
	maxPages int
	unit     string
}

func (c *issuesClosedCmd) Name() string { return "issues-closed" }
func (c *issuesClosedCmd) Description() string {
	return "Show closed issue volume over time"
}

func (c *issuesClosedCmd) RegisterFlags(fs *flag.FlagSet) {
	registerIssueWindowFlags(fs, &c.since, &c.period, &c.maxPages)
	fs.StringVar(&c.unit, "unit", "days", "duration unit: days|hours")
}

func (c *issuesClosedCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	period, err := normalizePeriod(c.period)
	if err != nil {
		return analysis.Dataset{}, err
	}
	unit, err := normalizeDurationUnit(c.unit)
	if err != nil {
		return analysis.Dataset{}, err
	}
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "closed",
		Sort:      "updated",
		Direction: "desc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssuesClosedDataset(issues, rc.Now.Add(-c.since), period, unit), nil
}

type issueBacklogCmd struct {
	maxPages int
	staleFor time.Duration
	sort     string
}

func (c *issueBacklogCmd) Name() string { return "issue-backlog" }
func (c *issueBacklogCmd) Description() string {
	return "List the oldest open issues in the backlog"
}

func (c *issueBacklogCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&c.maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
	fs.DurationVar(&c.staleFor, "stale-for", 30*24*time.Hour, "mark issues as stale after this age since last update")
	fs.StringVar(&c.sort, "sort", "age", "sort by: age|updated|comments")
}

func (c *issueBacklogCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	sortBy, err := normalizeIssueBacklogSort(c.sort)
	if err != nil {
		return analysis.Dataset{}, err
	}
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "open",
		Sort:      "created",
		Direction: "asc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssueBacklogDataset(issues, rc.Now, c.staleFor, sortBy), nil
}

type issueAgeCmd struct {
	maxPages int
}

func (c *issueAgeCmd) Name() string { return "issue-age" }
func (c *issueAgeCmd) Description() string {
	return "Show open issue age distribution"
}

func (c *issueAgeCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&c.maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
}

func (c *issueAgeCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "open",
		Sort:      "created",
		Direction: "asc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssueAgeDataset(issues, rc.Now), nil
}

type issueResolutionCmd struct {
	since    time.Duration
	maxPages int
	unit     string
	sort     string
}

func (c *issueResolutionCmd) Name() string { return "issue-resolution" }
func (c *issueResolutionCmd) Description() string {
	return "Measure issue resolution duration for closed issues"
}

func (c *issueResolutionCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.DurationVar(&c.since, "since", 30*24*time.Hour, "look back window for closed issues")
	fs.IntVar(&c.maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
	fs.StringVar(&c.unit, "unit", "days", "duration unit: days|hours")
	fs.StringVar(&c.sort, "sort", "duration", "sort by: duration|closed")
}

func (c *issueResolutionCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	unit, err := normalizeDurationUnit(c.unit)
	if err != nil {
		return analysis.Dataset{}, err
	}
	sortBy, err := normalizeResolutionSort(c.sort)
	if err != nil {
		return analysis.Dataset{}, err
	}
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "closed",
		Sort:      "updated",
		Direction: "desc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssueResolutionDataset(issues, rc.Now.Add(-c.since), unit, sortBy), nil
}

type issueResponseCmd struct {
	since       time.Duration
	maxPages    int
	commentPages int
	unit        string
}

func (c *issueResponseCmd) Name() string { return "issue-response" }
func (c *issueResponseCmd) Description() string {
	return "Measure time to first maintainer response on issues"
}

func (c *issueResponseCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.DurationVar(&c.since, "since", 30*24*time.Hour, "look back window for issues")
	fs.IntVar(&c.maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
	fs.IntVar(&c.commentPages, "comment-pages", 1, "max comment pages to fetch per issue (100 per page, 0 = all)")
	fs.StringVar(&c.unit, "unit", "days", "duration unit: days|hours")
}

func (c *issueResponseCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	unit, err := normalizeDurationUnit(c.unit)
	if err != nil {
		return analysis.Dataset{}, err
	}
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "all",
		Sort:      "created",
		Direction: "desc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	windowStart := rc.Now.Add(-c.since)
	filtered := filterIssueWindow(issues, windowStart)
	comments, err := fetchIssueComments(rc, filtered, c.commentPages)
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssueResponseDataset(filtered, comments, windowStart, unit), nil
}

type issueParticipantsCmd struct {
	since        time.Duration
	maxPages     int
	commentPages int
}

func (c *issueParticipantsCmd) Name() string { return "issue-participants" }
func (c *issueParticipantsCmd) Description() string {
	return "Show issues with the most distinct participants"
}

func (c *issueParticipantsCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.DurationVar(&c.since, "since", 30*24*time.Hour, "look back window for issues")
	fs.IntVar(&c.maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
	fs.IntVar(&c.commentPages, "comment-pages", 1, "max comment pages to fetch per issue (100 per page, 0 = all)")
}

func (c *issueParticipantsCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "all",
		Sort:      "updated",
		Direction: "desc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	windowStart := rc.Now.Add(-c.since)
	filtered := filterIssueWindow(issues, windowStart)
	comments, err := fetchIssueComments(rc, filtered, c.commentPages)
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssueParticipantsDataset(filtered, comments, windowStart), nil
}

type issueAbandonedCmd struct {
	maxPages     int
	commentPages int
	staleFor     time.Duration
}

func (c *issueAbandonedCmd) Name() string { return "issue-abandoned" }
func (c *issueAbandonedCmd) Description() string {
	return "Find stale open issues with no recent activity"
}

func (c *issueAbandonedCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&c.maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
	fs.IntVar(&c.commentPages, "comment-pages", 1, "max comment pages to fetch per issue (100 per page, 0 = all)")
	fs.DurationVar(&c.staleFor, "stale-for", 30*24*time.Hour, "consider issues abandoned after this long without updates")
}

func (c *issueAbandonedCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "open",
		Sort:      "updated",
		Direction: "asc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	comments, err := fetchIssueComments(rc, issues, c.commentPages)
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssueAbandonedDataset(issues, comments, rc.Now, c.staleFor), nil
}

type issueCountsCmd struct {
	since    time.Duration
	maxPages int
}

func (c *issueCountsCmd) Name() string { return "issue-counts" }
func (c *issueCountsCmd) Description() string {
	return "Show open and closed issue counts"
}

func (c *issueCountsCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.DurationVar(&c.since, "since", 30*24*time.Hour, "look back window used for recent count summaries")
	fs.IntVar(&c.maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
}

func (c *issueCountsCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
		MaxPages:  c.maxPages,
		State:     "all",
		Sort:      "updated",
		Direction: "desc",
	})
	if err != nil {
		return analysis.Dataset{}, err
	}
	return analysis.BuildIssueCountsDataset(issues, rc.Now.Add(-c.since)), nil
}

func registerIssueWindowFlags(fs *flag.FlagSet, since *time.Duration, period *string, maxPages *int) {
	fs.DurationVar(since, "since", 30*24*time.Hour, "look back window for issue analytics")
	fs.StringVar(period, "period", "week", "bucket period: day|week|month")
	fs.IntVar(maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
}

func normalizePeriod(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "day", "week", "month":
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", fmt.Errorf("unsupported -period=%q (expected day|week|month)", v)
	}
}

func normalizeDurationUnit(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "day", "days", "d":
		return "days", nil
	case "hour", "hours", "h":
		return "hours", nil
	default:
		return "", fmt.Errorf("unsupported -unit=%q (expected days|hours)", v)
	}
}

func normalizeIssueBacklogSort(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "age", "updated", "comments":
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", fmt.Errorf("unsupported -sort=%q (expected age|updated|comments)", v)
	}
}

func normalizeResolutionSort(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "duration", "closed":
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", fmt.Errorf("unsupported -sort=%q (expected duration|closed)", v)
	}
}

func filterIssueWindow(issues []platform.Issue, since time.Time) []platform.Issue {
	filtered := make([]platform.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		if issue.CreatedAt.IsZero() || issue.CreatedAt.Before(since) {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func fetchIssueComments(rc *RunContext, issues []platform.Issue, maxPages int) (map[int][]platform.IssueComment, error) {
	out := make(map[int][]platform.IssueComment, len(issues))
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		comments, err := rc.Client.ListIssueComments(rc.Ctx, rc.Owner, rc.Repo, issue.Number, maxPages)
		if err != nil {
			return nil, err
		}
		out[issue.Number] = comments
	}
	return out, nil
}
