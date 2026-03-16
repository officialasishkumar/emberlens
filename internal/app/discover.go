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
	discoverViewUntriaged       = "untriaged"
	discoverViewNeedsMaintainer = "needs-maintainer"
	discoverViewHotspots        = "hotspots"
)

type discoverViewSpec struct {
	name        string
	description string
}

var discoverViewSpecs = []discoverViewSpec{
	{name: discoverViewUntriaged, description: "open issues old enough to deserve triage but still without maintainer response"},
	{name: discoverViewNeedsMaintainer, description: "open issues with real discussion but still no maintainer engagement"},
	{name: discoverViewHotspots, description: "recently active open issues with concentrated discussion and participants"},
}

type discoverConfig struct {
	view            string
	since           time.Duration
	minAge          time.Duration
	maxPages        int
	commentPages    int
	minComments     int
	minParticipants int
	sort            string
}

func defaultDiscoverConfig(defaultView string) discoverConfig {
	return discoverConfig{
		view:            defaultView,
		since:           14 * 24 * time.Hour,
		minAge:          7 * 24 * time.Hour,
		maxPages:        5,
		commentPages:    1,
		minComments:     3,
		minParticipants: 3,
	}
}

type discoverCmd struct {
	cfg discoverConfig
}

func (c *discoverCmd) Name() string { return "discover" }
func (c *discoverCmd) Description() string {
	return "Find hidden issue patterns that are hard to spot in the web UI"
}

func (c *discoverCmd) RegisterFlags(fs *flag.FlagSet) {
	c.cfg = defaultDiscoverConfig(discoverViewUntriaged)
	registerDiscoverFlags(fs, &c.cfg)
	fs.Usage = func() {
		writeDiscoverUsage(fs.Output(), fs.Name())
		fs.PrintDefaults()
	}
}

func (c *discoverCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	return executeDiscoverView(c.cfg, rc)
}

func registerDiscoverFlags(fs *flag.FlagSet, cfg *discoverConfig) {
	fs.StringVar(&cfg.view, "view", cfg.view, "discover view: untriaged|needs-maintainer|hotspots")
	fs.DurationVar(&cfg.since, "since", cfg.since, "look back window for hotspots")
	fs.DurationVar(&cfg.minAge, "min-age", cfg.minAge, "minimum issue age for untriaged|needs-maintainer")
	fs.IntVar(&cfg.maxPages, "max-pages", cfg.maxPages, "max issue pages to fetch (100 per page, 0 = all)")
	fs.IntVar(&cfg.commentPages, "comment-pages", cfg.commentPages, "max comment pages to fetch per issue (100 per page, 0 = all)")
	fs.IntVar(&cfg.minComments, "min-comments", cfg.minComments, "minimum comment count for needs-maintainer|hotspots")
	fs.IntVar(&cfg.minParticipants, "min-participants", cfg.minParticipants, "minimum participant count for needs-maintainer|hotspots")
	fs.StringVar(&cfg.sort, "sort", "", "sort order depends on view")
}

func writeDiscoverUsage(w io.Writer, name string) {
	fmt.Fprintf(w, "Usage:\n  emberlens %s [flags]\n\n", name)
	io.WriteString(w, "Views:\n")
	for _, spec := range discoverViewSpecs {
		fmt.Fprintf(w, "  %-18s %s\n", spec.name, spec.description)
	}
	io.WriteString(w, "\nFlags:\n")
}

func executeDiscoverView(cfg discoverConfig, rc *RunContext) (analysis.Dataset, error) {
	view, err := normalizeDiscoverView(cfg.view)
	if err != nil {
		return analysis.Dataset{}, err
	}
	sortBy, err := normalizeDiscoverSort(view, cfg.sort)
	if err != nil {
		return analysis.Dataset{}, err
	}

	switch view {
	case discoverViewUntriaged:
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "open",
			Sort:      "created",
			Direction: "asc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		filtered := filterIssuesByMinAge(issues, rc.Now, cfg.minAge)
		comments, err := fetchIssueComments(rc, filtered, cfg.commentPages)
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildDiscoverUntriagedDataset(filtered, comments, rc.Now, cfg.minAge, sortBy), nil
	case discoverViewNeedsMaintainer:
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "open",
			Sort:      "updated",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		filtered := filterIssuesByMinAge(issues, rc.Now, cfg.minAge)
		filtered = filterIssuesByMinComments(filtered, cfg.minComments)
		comments, err := fetchIssueComments(rc, filtered, cfg.commentPages)
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildDiscoverNeedsMaintainerDataset(filtered, comments, rc.Now, cfg.minAge, cfg.minComments, cfg.minParticipants, sortBy), nil
	case discoverViewHotspots:
		issues, err := rc.Client.ListIssues(rc.Ctx, rc.Owner, rc.Repo, platform.IssueListOptions{
			MaxPages:  cfg.maxPages,
			State:     "open",
			Sort:      "updated",
			Direction: "desc",
		})
		if err != nil {
			return analysis.Dataset{}, err
		}
		windowStart := rc.Now.Add(-cfg.since)
		filtered := filterIssuesByUpdatedSince(issues, windowStart)
		filtered = filterIssuesByMinComments(filtered, cfg.minComments)
		comments, err := fetchIssueComments(rc, filtered, cfg.commentPages)
		if err != nil {
			return analysis.Dataset{}, err
		}
		return analysis.BuildDiscoverHotspotsDataset(filtered, comments, rc.Now, windowStart, cfg.minComments, cfg.minParticipants, sortBy), nil
	default:
		return analysis.Dataset{}, fmt.Errorf("unsupported -view=%q", cfg.view)
	}
}

func normalizeDiscoverView(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case discoverViewUntriaged, discoverViewNeedsMaintainer, discoverViewHotspots:
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", fmt.Errorf("unsupported -view=%q (expected untriaged|needs-maintainer|hotspots)", v)
	}
}

func normalizeDiscoverSort(view string, sort string) (string, error) {
	sort = strings.TrimSpace(sort)
	switch view {
	case discoverViewUntriaged:
		if sort == "" {
			sort = analysis.DiscoverSortAge
		}
		return normalizeDiscoverUntriagedSort(sort)
	case discoverViewNeedsMaintainer:
		if sort == "" {
			sort = analysis.DiscoverSortDiscussion
		}
		return normalizeDiscoverNeedsMaintainerSort(sort)
	case discoverViewHotspots:
		if sort == "" {
			sort = analysis.DiscoverSortHeat
		}
		return normalizeDiscoverHotspotSort(sort)
	default:
		return "", fmt.Errorf("unsupported discover view %q", view)
	}
}

func normalizeDiscoverUntriagedSort(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case analysis.DiscoverSortAge, analysis.DiscoverSortUpdated:
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", fmt.Errorf("unsupported -sort=%q (expected age|updated)", v)
	}
}

func normalizeDiscoverNeedsMaintainerSort(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case analysis.DiscoverSortDiscussion, analysis.DiscoverSortAge, analysis.DiscoverSortUpdated:
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", fmt.Errorf("unsupported -sort=%q (expected discussion|age|updated)", v)
	}
}

func normalizeDiscoverHotspotSort(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case analysis.DiscoverSortHeat, analysis.DiscoverSortUpdated, analysis.DiscoverSortComments:
		return strings.ToLower(strings.TrimSpace(v)), nil
	default:
		return "", fmt.Errorf("unsupported -sort=%q (expected heat|updated|comments)", v)
	}
}

func filterIssuesByMinAge(issues []platform.Issue, now time.Time, minAge time.Duration) []platform.Issue {
	filtered := make([]platform.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		if now.Sub(issue.CreatedAt) < minAge {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func filterIssuesByUpdatedSince(issues []platform.Issue, since time.Time) []platform.Issue {
	filtered := make([]platform.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		if issue.UpdatedAt.Before(since) {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func filterIssuesByMinComments(issues []platform.Issue, minComments int) []platform.Issue {
	filtered := make([]platform.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		if issue.Comments < minComments {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}
