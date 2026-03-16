package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/analysis"
	"github.com/officialasishkumar/emberlens/internal/display"
	"github.com/officialasishkumar/emberlens/internal/githubapi"
	"github.com/officialasishkumar/emberlens/internal/gitlabapi"
	"github.com/officialasishkumar/emberlens/internal/platform"
	"github.com/officialasishkumar/emberlens/internal/report"
)

// ---------------------------------------------------------------------------
// Subcommand interface — implement this to add a new emberlens command.
// ---------------------------------------------------------------------------

// Subcommand is the extension point for emberlens commands.
// Each command owns its own flags and execution logic.
// The Runner handles flag parsing, output rendering, and report saving.
type Subcommand interface {
	Name() string
	Description() string
	RegisterFlags(fs *flag.FlagSet)
	Execute(rc *RunContext) (analysis.Dataset, error)
}

// RunContext is the shared execution context passed to every command.
type RunContext struct {
	Ctx            context.Context
	Client         platform.Client
	Owner          string
	Repo           string
	SkipProfiles   bool
	Now            time.Time
	ProfileBaseURL string
}

// ---------------------------------------------------------------------------
// Runner
// ---------------------------------------------------------------------------

// Runner is the top-level CLI application.
type Runner struct {
	Stdout   io.Writer
	Stderr   io.Writer
	Now      func() time.Time
	commands map[string]Subcommand
	order    []string
}

// NewRunner creates a Runner with all built-in commands registered.
func NewRunner(stdout, stderr io.Writer) *Runner {
	r := &Runner{
		Stdout:   stdout,
		Stderr:   stderr,
		commands: map[string]Subcommand{},
	}
	r.Register(&contributorsCmd{})
	r.Register(&activeContributorsCmd{})
	r.Register(&maintainersCmd{})
	r.Register(&issuesNewCmd{})
	r.Register(&issuesActiveCmd{})
	r.Register(&issuesClosedCmd{})
	r.Register(&issueBacklogCmd{})
	r.Register(&issueAgeCmd{})
	r.Register(&issueResolutionCmd{})
	r.Register(&issueResponseCmd{})
	r.Register(&issueParticipantsCmd{})
	r.Register(&issueAbandonedCmd{})
	r.Register(&issueCountsCmd{})
	return r
}

// Register adds a command. Use this to extend emberlens with custom commands.
func (r *Runner) Register(cmd Subcommand) {
	r.commands[cmd.Name()] = cmd
	r.order = append(r.order, cmd.Name())
}

func (r *Runner) helpText() string {
	var b strings.Builder
	b.WriteString("emberlens — repository analytics from the CLI.\n\n")
	b.WriteString("Usage:\n  emberlens <command> [flags]\n\n")
	b.WriteString("Commands:\n")
	for _, name := range r.order {
		fmt.Fprintf(&b, "  %-24s %s\n", name, r.commands[name].Description())
	}
	b.WriteString(`
Global flags:
  -repo owner/repo         repository (required)
  -platform github|gitlab  platform (default: github)
  -token <token>           API token (or set GITHUB_TOKEN / GITLAB_TOKEN env var)
  -gitlab-url <url>        GitLab instance URL (default: https://gitlab.com)
  -output table|json       output format (default: table)
  -verbose                 show all fields in detailed card layout
  -limit N                 show only top N results (default: 20, 0 = all)
  -profiles                fetch full user profiles (extra API calls)
  -no-color                disable colored output
  -timeout <duration>      API timeout (default: 2m)
  -no-report               skip saving run report to disk
  -report-dir <dir>        report directory (default: emberlens-reports)

Use "emberlens <command> -h" for command-specific flags.
`)
	return b.String()
}

// Run parses arguments and executes the appropriate subcommand.
func (r *Runner) Run(args []string, envGitHubToken, envGitLabToken string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(r.Stdout, r.helpText())
		return 0
	}

	cmd, ok := r.commands[args[0]]
	if !ok {
		fmt.Fprintf(r.Stderr, "error: unknown command %q\n\n%s", args[0], r.helpText())
		return 2
	}

	// Parse common + command-specific flags
	fs := flag.NewFlagSet(cmd.Name(), flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	common := registerCommon(fs, envGitHubToken, envGitLabToken)
	cmd.RegisterFlags(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	common.postParse()

	owner, repo, err := parseRepo(common.repo)
	if err != nil {
		return r.fail(err)
	}

	// Setup
	dp := &display.Printer{W: r.Stdout, Color: !common.noColor}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), common.timeout)
	defer cancel()

	// Create platform client
	var client platform.Client
	switch strings.ToLower(common.platformName) {
	case "github":
		client = githubapi.NewClient(common.token)
	case "gitlab":
		client = gitlabapi.NewClient(common.token, common.gitlabURL)
	default:
		return r.fail(fmt.Errorf("unsupported -platform=%q (expected github|gitlab)", common.platformName))
	}

	rc := &RunContext{
		Ctx:            ctx,
		Client:         client,
		Owner:          owner,
		Repo:           repo,
		SkipProfiles:   !common.profiles,
		Now:            r.now(),
		ProfileBaseURL: client.ProfileBaseURL(),
	}

	// Execute
	result, err := cmd.Execute(rc)
	if err != nil {
		return r.fail(err)
	}

	result, total := result.CloneWithLimit(common.limit)
	elapsed := time.Since(start)

	// Render output
	if err := r.render(dp, result, common); err != nil {
		return r.fail(err)
	}

	// Save report
	var reportPath string
	if !common.noReport {
		cmdStr := reconstructCommand(cmd.Name(), fs)
		path, saveErr := report.Save(common.reportDir, common.repo, cmdStr, result, elapsed)
		if saveErr != nil {
			fmt.Fprintf(r.Stderr, "warning: failed to save report: %v\n", saveErr)
		} else {
			reportPath = path
		}
	}

	// Footer summary (table output only)
	if common.output == "table" {
		var lines []string
		if common.limit > 0 && common.limit < total {
			lines = append(lines, fmt.Sprintf("Showing %d of %d rows · %s", len(result.Records), total, elapsed.Round(time.Millisecond)))
		} else {
			lines = append(lines, fmt.Sprintf("%d rows · %s", total, elapsed.Round(time.Millisecond)))
		}
		lines = append(lines, result.Hints...)
		if reportPath != "" {
			lines = append(lines, "Report: "+reportPath)
		}
		dp.Footer(lines...)
	}

	return 0
}

// ---------------------------------------------------------------------------
// Common flags
// ---------------------------------------------------------------------------

type commonFlags struct {
	repo            string
	platformName    string
	token           string
	gitlabURL       string
	output          string
	verbose         bool
	limit           int
	profiles        bool
	noColor         bool
	timeout         time.Duration
	noReport        bool
	reportDir       string
	envGitHubToken  string
	envGitLabToken  string
}

func registerCommon(fs *flag.FlagSet, envGitHubToken, envGitLabToken string) *commonFlags {
	f := &commonFlags{}
	fs.StringVar(&f.repo, "repo", "", "repository in owner/repo format (required)")
	fs.StringVar(&f.platformName, "platform", "github", "platform: github|gitlab")
	fs.StringVar(&f.token, "token", "", "API token (defaults to GITHUB_TOKEN or GITLAB_TOKEN)")
	fs.StringVar(&f.gitlabURL, "gitlab-url", "", "GitLab instance URL (default: https://gitlab.com)")
	fs.StringVar(&f.output, "output", "table", "output format: table|json")
	fs.BoolVar(&f.verbose, "verbose", false, "show all fields in detailed card layout")
	fs.IntVar(&f.limit, "limit", 20, "show only top N results (0 = all)")
	fs.BoolVar(&f.profiles, "profiles", false, "fetch full user profiles (extra API calls)")
	fs.BoolVar(&f.noColor, "no-color", false, "disable colored output")
	fs.DurationVar(&f.timeout, "timeout", 2*time.Minute, "API timeout duration")
	fs.BoolVar(&f.noReport, "no-report", false, "skip saving run report to disk")
	fs.StringVar(&f.reportDir, "report-dir", "emberlens-reports", "report directory")

	// Store env tokens for post-parse resolution.
	f.envGitHubToken = envGitHubToken
	f.envGitLabToken = envGitLabToken
	return f
}

// postParse resolves the token after flag parsing, using env vars as fallback.
func (f *commonFlags) postParse() {
	if f.token == "" {
		switch strings.ToLower(f.platformName) {
		case "gitlab":
			f.token = f.envGitLabToken
		default:
			f.token = f.envGitHubToken
		}
	}
}

// ---------------------------------------------------------------------------
// Output rendering
// ---------------------------------------------------------------------------

func (r *Runner) render(dp *display.Printer, result analysis.Dataset, common *commonFlags) error {
	switch common.output {
	case "table":
		dp.Banner("emberlens", common.repo, result.Title)
		dp.Stats(toDisplayStats(result.Summary))
		if common.verbose {
			r.printCards(dp, result)
		} else {
			r.printTable(dp, result)
		}
		return nil
	case "json":
		enc := json.NewEncoder(r.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	default:
		return fmt.Errorf("unsupported -output=%q (expected table|json)", common.output)
	}
}

func (r *Runner) printTable(dp *display.Printer, result analysis.Dataset) {
	headers := make([]string, 0, len(result.Columns))
	for _, column := range result.Columns {
		headers = append(headers, column.Label)
	}
	rows := make([][]string, 0, len(result.Records))
	for _, record := range result.Records {
		row := make([]string, 0, len(result.Columns))
		for _, column := range result.Columns {
			row = append(row, analysis.StringValue(record[column.Key]))
		}
		rows = append(rows, row)
	}
	dp.Table(headers, rows)
}

func (r *Runner) printCards(dp *display.Printer, result analysis.Dataset) {
	for i, record := range result.Records {
		dp.Card(i+1, buildCardFields(result, record))
	}
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func (r *Runner) fail(err error) int {
	fmt.Fprintf(r.Stderr, "error: %v\n", err)
	return 1
}

func (r *Runner) now() time.Time {
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

func reconstructCommand(name string, fs *flag.FlagSet) string {
	parts := []string{"emberlens", name}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "token" {
			return
		}
		parts = append(parts, fmt.Sprintf("-%s=%s", f.Name, f.Value.String()))
	})
	return strings.Join(parts, " ")
}

func emptyDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

func buildCardFields(result analysis.Dataset, record map[string]any) []display.CardField {
	fields := make([]display.CardField, 0, len(record))
	seen := map[string]struct{}{}
	for _, column := range result.Columns {
		fields = append(fields, display.CardField{
			Label: column.Label,
			Value: analysis.StringValue(record[column.Key]),
		})
		seen[column.Key] = struct{}{}
	}

	extraKeys := make([]string, 0, len(record))
	for key := range record {
		if _, ok := seen[key]; ok {
			continue
		}
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)
	for _, key := range extraKeys {
		fields = append(fields, display.CardField{
			Label: formatFieldLabel(key),
			Value: analysis.StringValue(record[key]),
		})
	}
	return fields
}

func toDisplayStats(stats []analysis.Stat) []display.Stat {
	out := make([]display.Stat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, display.Stat{Label: stat.Label, Value: stat.Value})
	}
	return out
}

func formatFieldLabel(key string) string {
	parts := strings.Fields(strings.ReplaceAll(key, "_", " "))
	for i, part := range parts {
		if strings.EqualFold(part, "url") {
			parts[i] = "URL"
			continue
		}
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}
