package app

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/analysis"
	"github.com/officialasishkumar/emberlens/internal/platform"
)

const (
	issueViewNew          = "new"
	issueViewActive       = "active"
	issueViewClosed       = "closed"
	issueViewBacklog      = "backlog"
	issueViewAge          = "age"
	issueViewResolution   = "resolution"
	issueViewResponse     = "response"
	issueViewParticipants = "participants"
	issueViewAbandoned    = "abandoned"
	issueViewCounts       = "counts"
)

type issueViewSpec struct {
	name        string
	description string
}

var issueViewSpecs = []issueViewSpec{
	{name: issueViewCounts, description: "open and closed issue inventory plus recent totals"},
	{name: issueViewNew, description: "new issue volume over time"},
	{name: issueViewActive, description: "issue activity over time based on last update"},
	{name: issueViewClosed, description: "closed issue volume plus average resolution summary"},
	{name: issueViewBacklog, description: "oldest open issues in the backlog"},
	{name: issueViewAge, description: "open issue age distribution"},
	{name: issueViewResolution, description: "resolution duration for recently closed issues"},
	{name: issueViewResponse, description: "first maintainer response latency"},
	{name: issueViewParticipants, description: "issues with the most distinct participants"},
	{name: issueViewAbandoned, description: "stale open issues with no recent activity"},
}

type issuesConfig struct {
	view         string
	since        time.Duration
	period       string
	maxPages     int
	unit         string
	sort         string
	staleFor     time.Duration
	commentPages int
}

func defaultIssuesConfig(defaultView string) issuesConfig {
	return issuesConfig{
		view:         defaultView,
		since:        30 * 24 * time.Hour,
		period:       "week",
		maxPages:     5,
		unit:         "days",
		staleFor:     30 * 24 * time.Hour,
		commentPages: 1,
	}
}

type issuesCmd struct {
	cfg issuesConfig
}

func (c *issuesCmd) Name() string { return "issues" }
func (c *issuesCmd) Description() string {
	return "Issue analytics selected with -view"
}

func (c *issuesCmd) RegisterFlags(fs *flag.FlagSet) {
	c.cfg = defaultIssuesConfig(issueViewCounts)
	registerIssuesAggregateFlags(fs, &c.cfg)
	fs.Usage = func() {
		writeIssuesUsage(fs.Output(), fs.Name())
		fs.PrintDefaults()
	}
}

func (c *issuesCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	return executeIssuesView(c.cfg, rc)
}

type legacyIssueAliasCmd struct {
	name        string
	description string
	view        string
	cfg         issuesConfig
}

func newLegacyIssueAliasCmd(name, description, view string) *legacyIssueAliasCmd {
	return &legacyIssueAliasCmd{
		name:        name,
		description: description,
		view:        view,
	}
}

func (c *legacyIssueAliasCmd) Hidden() bool { return true }
func (c *legacyIssueAliasCmd) Name() string { return c.name }
func (c *legacyIssueAliasCmd) Description() string {
	return c.description
}

func (c *legacyIssueAliasCmd) RegisterFlags(fs *flag.FlagSet) {
	c.cfg = defaultIssuesConfig(c.view)
	registerIssueFlagsForView(fs, &c.cfg, c.view)
}

func (c *legacyIssueAliasCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	cfg := c.cfg
	cfg.view = c.view
	return executeIssuesView(cfg, rc)
}

func registerIssuesAggregateFlags(fs *flag.FlagSet, cfg *issuesConfig) {
	fs.StringVar(&cfg.view, "view", cfg.view, "issue view: counts|new|active|closed|backlog|age|resolution|response|participants|abandoned")
	fs.DurationVar(&cfg.since, "since", cfg.since, "look back window for issue analytics")
	fs.StringVar(&cfg.period, "period", cfg.period, "bucket period for new|active|closed views: day|week|month")
	fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
	fs.StringVar(&cfg.unit, "unit", cfg.unit, "duration unit for closed|resolution|response views: days|hours")
	fs.StringVar(&cfg.sort, "sort", "", "sort order for backlog|resolution views")
	fs.DurationVar(&cfg.staleFor, "stale-for", cfg.staleFor, "staleness threshold for backlog|abandoned views")
	fs.IntVar(&cfg.commentPages, "comment-pages", cfg.commentPages, "max comment pages to fetch per issue for response|participants|abandoned views (100 per page, 0 = all)")
}

func registerIssueFlagsForView(fs *flag.FlagSet, cfg *issuesConfig, view string) {
	switch view {
	case issueViewNew, issueViewActive:
		registerIssueWindowFlags(fs, &cfg.since, &cfg.period, &cfg.maxPages)
	case issueViewClosed:
		registerIssueWindowFlags(fs, &cfg.since, &cfg.period, &cfg.maxPages)
		fs.StringVar(&cfg.unit, "unit", cfg.unit, "duration unit: days|hours")
	case issueViewBacklog:
		fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
		fs.DurationVar(&cfg.staleFor, "stale-for", cfg.staleFor, "mark issues as stale after this age since last update")
		fs.StringVar(&cfg.sort, "sort", "age", "sort by: age|updated|comments")
	case issueViewAge:
		fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
	case issueViewResolution:
		fs.DurationVar(&cfg.since, "since", cfg.since, "look back window for closed issues")
		fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
		fs.StringVar(&cfg.unit, "unit", cfg.unit, "duration unit: days|hours")
		fs.StringVar(&cfg.sort, "sort", "duration", "sort by: duration|closed")
	case issueViewResponse:
		fs.DurationVar(&cfg.since, "since", cfg.since, "look back window for issues")
		fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
		fs.IntVar(&cfg.commentPages, "comment-pages", cfg.commentPages, "max comment pages to fetch per issue (100 per page, 0 = all)")
		fs.StringVar(&cfg.unit, "unit", cfg.unit, "duration unit: days|hours")
	case issueViewParticipants:
		fs.DurationVar(&cfg.since, "since", cfg.since, "look back window for issues")
		fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
		fs.IntVar(&cfg.commentPages, "comment-pages", cfg.commentPages, "max comment pages to fetch per issue (100 per page, 0 = all)")
	case issueViewAbandoned:
		fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
		fs.IntVar(&cfg.commentPages, "comment-pages", cfg.commentPages, "max comment pages to fetch per issue (100 per page, 0 = all)")
		fs.DurationVar(&cfg.staleFor, "stale-for", cfg.staleFor, "consider issues abandoned after this long without updates")
	case issueViewCounts:
		fs.DurationVar(&cfg.since, "since", cfg.since, "look back window used for recent count summaries")
		fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
	}
}

func writeIssuesUsage(w io.Writer, name string) {
	fmt.Fprintf(w, "Usage:\n  emberlens %s [flags]\n\n", name)
	io.WriteString(w, "Views:\n")
	for _, spec := range issueViewSpecs {
		fmt.Fprintf(w, "  %-13s %s\n", spec.name, spec.description)
	}
	io.WriteString(w, "\nFlags:\n")
}

func executeIssuesView(cfg issuesConfig, rc *RunContext) (analysis.Dataset, error) {
	view, err := normalizeIssueView(cfg.view)
	if err != nil {
		return analysis.Dataset{}, err
	}
	sortBy, err := normalizeIssueSort(view, cfg.sort)
	if err != nil {
		return analysis.Dataset{}, err
	}

	switch view {
	case issueViewNew:
		period, err := normalizePeriod(cfg.period)
		if err != nil {
			return analysis.Dataset{}, err
		}
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "all",
			Sort:      "created",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssuesNewDataset(issues, rc.Now.Add(-cfg.since), period), nil
	case issueViewActive:
		period, err := normalizePeriod(cfg.period)
		if err != nil {
			return analysis.Dataset{}, err
		}
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "all",
			Sort:      "updated",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssuesActiveDataset(issues, rc.Now.Add(-cfg.since), period), nil
	case issueViewClosed:
		period, err := normalizePeriod(cfg.period)
		if err != nil {
			return analysis.Dataset{}, err
		}
		unit, err := normalizeDurationUnit(cfg.unit)
		if err != nil {
			return analysis.Dataset{}, err
		}
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "closed",
			Sort:      "updated",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssuesClosedDataset(issues, rc.Now.Add(-cfg.since), period, unit), nil
	case issueViewBacklog:
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "open",
			Sort:      "created",
			Direction: "asc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssueBacklogDataset(issues, rc.Now, cfg.staleFor, sortBy), nil
	case issueViewAge:
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "open",
			Sort:      "created",
			Direction: "asc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssueAgeDataset(issues, rc.Now), nil
	case issueViewResolution:
		unit, err := normalizeDurationUnit(cfg.unit)
		if err != nil {
			return analysis.Dataset{}, err
		}
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "closed",
			Sort:      "updated",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssueResolutionDataset(issues, rc.Now.Add(-cfg.since), unit, sortBy), nil
	case issueViewResponse:
		unit, err := normalizeDurationUnit(cfg.unit)
		if err != nil {
			return analysis.Dataset{}, err
		}
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "all",
			Sort:      "created",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		windowStart := rc.Now.Add(-cfg.since)
		filtered := filterIssueWindow(issues, windowStart)
		comments, err := fetchIssueComments(rc, filtered, cfg.commentPages)
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssueResponseDataset(filtered, comments, windowStart, unit), nil
	case issueViewParticipants:
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "all",
			Sort:      "updated",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		windowStart := rc.Now.Add(-cfg.since)
		filtered := filterIssueWindow(issues, windowStart)
		comments, err := fetchIssueComments(rc, filtered, cfg.commentPages)
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssueParticipantsDataset(filtered, comments, windowStart), nil
	case issueViewAbandoned:
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "open",
			Sort:      "updated",
			Direction: "asc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		comments, err := fetchIssueComments(rc, issues, cfg.commentPages)
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssueAbandonedDataset(issues, comments, rc.Now, cfg.staleFor), nil
	case issueViewCounts:
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "all",
			Sort:      "updated",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildIssueCountsDataset(issues, rc.Now.Add(-cfg.since)), nil
	default:
		return analysis.Dataset{}, fmt.Errorf("unsupported -view=%q", cfg.view)
	}
}

func registerIssueWindowFlags(fs *flag.FlagSet, since *time.Duration, period *string, maxPages *int) {
	fs.DurationVar(since, "since", 30*24*time.Hour, "look back window for issue analytics")
	fs.StringVar(period, "period", "week", "bucket period: day|week|month")
	fs.IntVar(maxPages, "max-pages", 5, "max issue pages to fetch (100 per page, 0 = all)")
}

func normalizeIssueView(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case issueViewNew, issueViewActive, issueViewClosed, issueViewBacklog, issueViewAge, issueViewResolution, issueViewResponse, issueViewParticipants, issueViewAbandoned, issueViewCounts:
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", fmt.Errorf("unsupported -view=%q (expected counts|new|active|closed|backlog|age|resolution|response|participants|abandoned)", v)
	}
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

func normalizeIssueSort(view, sort string) (string, error) {
	sort = strings.TrimSpace(sort)
	switch view {
	case issueViewBacklog:
		if sort == "" {
			sort = "age"
		}
		return normalizeIssueBacklogSort(sort)
	case issueViewResolution:
		if sort == "" {
			sort = "duration"
		}
		return normalizeResolutionSort(sort)
	default:
		if sort != "" {
			return "", fmt.Errorf("-sort is only supported with -view=%s or -view=%s", issueViewBacklog, issueViewResolution)
		}
		return "", nil
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
