package app

import (
	"context"
	"flag"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/analysis"
	"github.com/officialasishkumar/emberlens/internal/githubapi"
)

// =============================================================================
// contributors
// =============================================================================

type contributorsCmd struct {
	maxPages int
}

func (c *contributorsCmd) Name() string { return "contributors" }
func (c *contributorsCmd) Description() string {
	return "List all-time contributors ranked by contributions"
}

func (c *contributorsCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&c.maxPages, "max-pages", 3, "max contributor pages to fetch (100 per page)")
}

func (c *contributorsCmd) Execute(rc *RunContext) ([]analysis.Person, error) {
	contributors, err := rc.Client.ListContributors(rc.Ctx, rc.Owner, rc.Repo, c.maxPages)
	if err != nil {
		return nil, err
	}

	var profiles map[string]githubapi.Profile
	if !rc.SkipProfiles {
		profiles = fetchProfiles(rc.Ctx, rc.Client, contributorLogins(contributors))
	}

	return analysis.BuildContributors(contributors, profiles), nil
}

// =============================================================================
// active-contributors
// =============================================================================

type activeContributorsCmd struct {
	since       time.Duration
	commitPages int
}

func (c *activeContributorsCmd) Name() string { return "active-contributors" }
func (c *activeContributorsCmd) Description() string {
	return "List contributors active within a time window"
}

func (c *activeContributorsCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.DurationVar(&c.since, "since", 30*24*time.Hour, "time window for activity (e.g. 720h, 168h)")
	fs.IntVar(&c.commitPages, "commit-pages", 5, "max commit pages to fetch (100 per page)")
}

func (c *activeContributorsCmd) Execute(rc *RunContext) ([]analysis.Person, error) {
	sinceTs := rc.Now.Add(-c.since)
	commits, err := rc.Client.ListCommitsSince(rc.Ctx, rc.Owner, rc.Repo, sinceTs, c.commitPages)
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, cm := range commits {
		if cm.Author == nil || strings.TrimSpace(cm.Author.Login) == "" {
			continue
		}
		counts[cm.Author.Login]++
	}

	var profiles map[string]githubapi.Profile
	if !rc.SkipProfiles {
		profiles = fetchProfiles(rc.Ctx, rc.Client, mapKeys(counts))
	}

	return analysis.BuildActiveContributors(counts, profiles), nil
}

// =============================================================================
// maintainers
// =============================================================================

type maintainersCmd struct {
	minContrib   int
	topPercent   float64
	signalWeight int
	signalPages  int
	signals      bool
	maxPages     int
}

func (c *maintainersCmd) Name() string { return "maintainers" }
func (c *maintainersCmd) Description() string {
	return "Identify likely maintainers via contributions + team signals"
}

func (c *maintainersCmd) RegisterFlags(fs *flag.FlagSet) {
	fs.IntVar(&c.minContrib, "min-contributions", 25, "minimum all-time contributions")
	fs.Float64Var(&c.topPercent, "top-percent", 0.02, "top contribution share threshold (0-1)")
	fs.IntVar(&c.signalWeight, "signal-weight", 25, "score weight per team signal")
	fs.IntVar(&c.signalPages, "signal-pages", 3, "PR/issue pages for signal detection")
	fs.BoolVar(&c.signals, "signals", false, "enable team signal detection (extra API calls)")
	fs.IntVar(&c.maxPages, "max-pages", 3, "max contributor pages to fetch (0 = all)")
}

func (c *maintainersCmd) Execute(rc *RunContext) ([]analysis.Person, error) {
	repoMeta, err := rc.Client.GetRepo(rc.Ctx, rc.Owner, rc.Repo)
	if err != nil {
		return nil, err
	}

	contributors, err := rc.Client.ListContributors(rc.Ctx, rc.Owner, rc.Repo, c.maxPages)
	if err != nil {
		return nil, err
	}

	var teamSignals map[string][]string
	if c.signals {
		teamSignals = collectTeamSignals(rc.Ctx, rc.Client, rc.Owner, rc.Repo, repoMeta.Owner.Type, c.signalPages)
	}

	logins := contributorLogins(contributors)
	if len(teamSignals) > 0 {
		logins = mergeLogins(logins, mapKeys(teamSignals))
	}

	var profiles map[string]githubapi.Profile
	if !rc.SkipProfiles {
		profiles = fetchProfiles(rc.Ctx, rc.Client, logins)
	}

	return analysis.BuildMaintainers(contributors, teamSignals, profiles, analysis.MaintainerConfig{
		MinContributions: c.minContrib,
		TopPercent:       c.topPercent,
		SignalWeight:     c.signalWeight,
	})
}

// =============================================================================
// Shared helpers for GitHub API interactions
// =============================================================================

func collectTeamSignals(ctx context.Context, client *githubapi.Client, owner, repo, ownerType string, maxPages int) map[string][]string {
	collected := map[string]map[string]struct{}{}
	add := func(login, reason string) {
		if strings.TrimSpace(login) == "" {
			return
		}
		if _, ok := collected[login]; !ok {
			collected[login] = map[string]struct{}{}
		}
		collected[login][reason] = struct{}{}
	}

	prs, err := client.ListPullRequests(ctx, owner, repo, maxPages)
	if err == nil {
		for _, pr := range prs {
			if isTeamAssociation(pr.AuthorAssociation) {
				add(pr.User.Login, "PR author association="+pr.AuthorAssociation)
			}
		}
	}

	issues, err := client.ListIssues(ctx, owner, repo, maxPages)
	if err == nil {
		for _, issue := range issues {
			if issue.PullRequest != nil {
				continue
			}
			if isTeamAssociation(issue.AuthorAssociation) {
				add(issue.User.Login, "Issue author association="+issue.AuthorAssociation)
			}
		}
	}

	if ownerType == "Organization" {
		members, err := client.ListPublicOrgMembers(ctx, owner)
		if err == nil {
			for _, m := range members {
				add(m.Login, "Public organization member")
			}
		}
	}

	out := make(map[string][]string, len(collected))
	for login, reasons := range collected {
		out[login] = mapKeys(reasons)
	}
	return out
}

func isTeamAssociation(v string) bool {
	switch strings.ToUpper(v) {
	case "OWNER", "MEMBER", "COLLABORATOR":
		return true
	default:
		return false
	}
}

func fetchProfiles(ctx context.Context, client *githubapi.Client, logins []string) map[string]githubapi.Profile {
	out := map[string]githubapi.Profile{}
	for _, login := range dedupe(logins) {
		p, err := client.GetProfile(ctx, login)
		if err == nil {
			out[login] = p
		}
	}
	return out
}

func contributorLogins(in []githubapi.Contributor) []string {
	logins := make([]string, 0, len(in))
	for _, c := range in {
		if c.Login != "" {
			logins = append(logins, c.Login)
		}
	}
	return logins
}

func mergeLogins(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

func dedupe(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
