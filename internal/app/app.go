package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/officialasishkumar/emberlens/internal/analysis"
	"github.com/officialasishkumar/emberlens/internal/githubapi"
)

const helpText = `emberlens: GitHub repository people analytics from CLI.

Usage:
  emberlens <command> [flags]

Commands:
  contributors         List all-time contributors ranked by contributions
  active-contributors  List contributors active within a period (via commit activity)
  maintainers          Identify likely maintainers using contribution + team signals

Global conventions:
  -repo owner/repo         required on every command
  -token <token>           optional (or set GITHUB_TOKEN)
  -output table|json       default table

Use "emberlens <command> -h" for command-specific flags.
`

type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
	Now    func() time.Time
}

func (r Runner) Run(args []string, envToken string) int {
	if len(args) == 0 {
		fmt.Fprint(r.Stdout, helpText)
		return 0
	}

	switch args[0] {
	case "contributors":
		return r.runContributors(args[1:], envToken)
	case "active-contributors":
		return r.runActiveContributors(args[1:], envToken)
	case "maintainers":
		return r.runMaintainers(args[1:], envToken)
	case "help", "-h", "--help":
		fmt.Fprint(r.Stdout, helpText)
		return 0
	default:
		fmt.Fprintf(r.Stderr, "error: unknown command %q\n\n%s", args[0], helpText)
		return 2
	}
}

type commonFlags struct {
	repo   string
	token  string
	output string
}

func registerCommon(fs *flag.FlagSet, envToken string) *commonFlags {
	f := &commonFlags{}
	fs.StringVar(&f.repo, "repo", "", "repository in owner/repo format (required)")
	fs.StringVar(&f.token, "token", envToken, "GitHub token (defaults to GITHUB_TOKEN)")
	fs.StringVar(&f.output, "output", "table", "output format: table|json")
	return f
}

func (r Runner) runContributors(args []string, envToken string) int {
	fs := flag.NewFlagSet("contributors", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	common := registerCommon(fs, envToken)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	owner, repo, err := parseRepo(common.repo)
	if err != nil {
		return r.fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := githubapi.NewClient(common.token)

	contributors, err := client.ListContributors(ctx, owner, repo)
	if err != nil {
		return r.fail(err)
	}
	profiles := fetchProfiles(ctx, client, contributorLogins(contributors))

	people := analysis.BuildContributors(contributors, profiles)
	return r.print(people, common.output)
}

func (r Runner) runActiveContributors(args []string, envToken string) int {
	fs := flag.NewFlagSet("active-contributors", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	common := registerCommon(fs, envToken)
	since := fs.Duration("since", 30*24*time.Hour, "time window for active contributions (example: 720h, 168h)")
	commitPages := fs.Int("commit-pages", 5, "max commit pages (100/page) to fetch for activity")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	owner, repo, err := parseRepo(common.repo)
	if err != nil {
		return r.fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := githubapi.NewClient(common.token)

	sinceTs := r.now().Add(-*since)
	commits, err := client.ListCommitsSince(ctx, owner, repo, sinceTs, *commitPages)
	if err != nil {
		return r.fail(err)
	}

	counts := map[string]int{}
	for _, c := range commits {
		if c.Author == nil || strings.TrimSpace(c.Author.Login) == "" {
			continue
		}
		counts[c.Author.Login]++
	}

	profiles := fetchProfiles(ctx, client, mapKeys(counts))
	people := analysis.BuildActiveContributors(counts, profiles)
	return r.print(people, common.output)
}

func (r Runner) runMaintainers(args []string, envToken string) int {
	fs := flag.NewFlagSet("maintainers", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	common := registerCommon(fs, envToken)
	minContrib := fs.Int("min-contributions", 25, "minimum all-time contributions")
	topPercent := fs.Float64("top-percent", 0.02, "share of total all-time contributions threshold (0-1)")
	signalWeight := fs.Int("signal-weight", 25, "weight added per team signal")
	signalPages := fs.Int("signal-pages", 3, "PR/issue pages to inspect for team signals")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	owner, repo, err := parseRepo(common.repo)
	if err != nil {
		return r.fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := githubapi.NewClient(common.token)

	repoMeta, err := client.GetRepo(ctx, owner, repo)
	if err != nil {
		return r.fail(err)
	}

	contributors, err := client.ListContributors(ctx, owner, repo)
	if err != nil {
		return r.fail(err)
	}
	teamSignals := collectTeamSignals(ctx, client, owner, repo, repoMeta.Owner.Type, *signalPages)
	profiles := fetchProfiles(ctx, client, mergeLogins(contributorLogins(contributors), mapKeys(teamSignals)))

	people, err := analysis.BuildMaintainers(contributors, teamSignals, profiles, analysis.MaintainerConfig{
		MinContributions: *minContrib,
		TopPercent:       *topPercent,
		SignalWeight:     *signalWeight,
	})
	if err != nil {
		return r.fail(err)
	}
	return r.print(people, common.output)
}

func (r Runner) print(people []analysis.Person, output string) int {
	switch output {
	case "table":
		tw := tabwriter.NewWriter(r.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "LOGIN\tNAME\tCONTRIBUTIONS\tPROFILE\tLINKS\tSIGNALS\tREASONS")
		for _, p := range people {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
				p.Login,
				emptyDash(p.Name),
				p.Contributions,
				p.ProfileURL,
				emptyDash(strings.Join(p.ExternalLinks, ", ")),
				emptyDash(strings.Join(p.Signals, "; ")),
				emptyDash(strings.Join(p.Reasons, " | ")),
			)
		}
		_ = tw.Flush()
		return 0
	case "json":
		enc := json.NewEncoder(r.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(people); err != nil {
			return r.fail(err)
		}
		return 0
	default:
		return r.fail(fmt.Errorf("unsupported -output=%q (expected table|json)", output))
	}
}

func (r Runner) fail(err error) int {
	fmt.Fprintf(r.Stderr, "error: %v\n", err)
	return 1
}

func (r Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func parseRepo(v string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(v), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("-repo must be owner/repo")
	}
	return parts[0], parts[1], nil
}

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

func emptyDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
