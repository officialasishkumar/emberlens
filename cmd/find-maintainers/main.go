package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/example/find-maintainers/internal/analysis"
	"github.com/example/find-maintainers/internal/githubapi"
)

func main() {
	var repoFlag string
	var tokenFlag string
	var minContrib int
	var topPercent float64
	var output string
	var signalPages int

	flag.StringVar(&repoFlag, "repo", "", "GitHub repository in owner/repo format")
	flag.StringVar(&tokenFlag, "token", os.Getenv("GITHUB_TOKEN"), "GitHub token (defaults to GITHUB_TOKEN)")
	flag.IntVar(&minContrib, "min-contributions", 25, "Minimum commit contributions required")
	flag.Float64Var(&topPercent, "top-percent", 0.02, "Repo-share contribution threshold (0-1)")
	flag.StringVar(&output, "output", "table", "Output format: table or json")
	flag.IntVar(&signalPages, "signal-pages", 3, "Number of pages to inspect for PR/issue team signals")
	flag.Parse()

	owner, repo, err := parseRepo(repoFlag)
	if err != nil {
		exitf("invalid -repo: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client := githubapi.NewClient(tokenFlag)
	repoMeta, err := client.GetRepo(ctx, owner, repo)
	if err != nil {
		exitf("fetch repo metadata: %v", err)
	}

	contributors, err := client.ListContributors(ctx, owner, repo)
	if err != nil {
		exitf("fetch contributors: %v", err)
	}

	signals := collectTeamSignals(ctx, client, owner, repo, repoMeta.Owner.Type, signalPages)
	profiles := fetchProfiles(ctx, client, contributors, signals)

	candidates, err := analysis.BuildCandidates(contributors, signals, profiles, analysis.Config{
		MinContributions: minContrib,
		TopPercent:       topPercent,
	})
	if err != nil {
		exitf("build maintainers: %v", err)
	}

	switch output {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(candidates); err != nil {
			exitf("write json output: %v", err)
		}
	case "table":
		printTable(candidates)
	default:
		exitf("unsupported output format %q (expected table|json)", output)
	}
}

func collectTeamSignals(ctx context.Context, client *githubapi.Client, owner, repo, ownerType string, pages int) map[string][]string {
	store := map[string]map[string]struct{}{}
	add := func(login, reason string) {
		if strings.TrimSpace(login) == "" {
			return
		}
		if _, ok := store[login]; !ok {
			store[login] = map[string]struct{}{}
		}
		store[login][reason] = struct{}{}
	}

	prs, err := client.ListPullRequests(ctx, owner, repo, pages)
	if err == nil {
		for _, pr := range prs {
			if isTeamAssociation(pr.AuthorAssociation) {
				add(pr.User.Login, "PR author association="+pr.AuthorAssociation)
			}
		}
	}

	issues, err := client.ListIssues(ctx, owner, repo, pages)
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

	out := make(map[string][]string, len(store))
	for login, set := range store {
		out[login] = make([]string, 0, len(set))
		for reason := range set {
			out[login] = append(out[login], reason)
		}
	}
	return out
}

func fetchProfiles(ctx context.Context, client *githubapi.Client, contributors []githubapi.Contributor, signals map[string][]string) map[string]githubapi.Profile {
	profiles := map[string]githubapi.Profile{}
	seen := map[string]struct{}{}
	for _, c := range contributors {
		seen[c.Login] = struct{}{}
	}
	for login := range signals {
		seen[login] = struct{}{}
	}
	for login := range seen {
		if strings.TrimSpace(login) == "" {
			continue
		}
		p, err := client.GetProfile(ctx, login)
		if err == nil {
			profiles[login] = p
		}
	}
	return profiles
}

func isTeamAssociation(value string) bool {
	switch strings.ToUpper(value) {
	case "MEMBER", "OWNER", "COLLABORATOR":
		return true
	default:
		return false
	}
}

func parseRepo(repo string) (owner, name string, err error) {
	parts := strings.Split(strings.TrimSpace(repo), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected owner/repo")
	}
	return parts[0], parts[1], nil
}

func printTable(candidates []analysis.Candidate) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "LOGIN\tNAME\tCOMMITS\tPROFILE\tEXTERNAL LINKS\tTEAM SIGNALS")
	for _, c := range candidates {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n",
			c.Login,
			emptyDash(c.Name),
			c.Contributions,
			c.ProfileURL,
			emptyDash(strings.Join(c.ExternalLinks, ", ")),
			emptyDash(strings.Join(c.TeamSignals, "; ")),
		)
	}
	_ = tw.Flush()
}

func emptyDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
