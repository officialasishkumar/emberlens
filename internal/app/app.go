package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/analysis"
	"github.com/officialasishkumar/emberlens/internal/display"
	"github.com/officialasishkumar/emberlens/internal/githubapi"
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
	Execute(rc *RunContext) ([]analysis.Person, error)
}

// RunContext is the shared execution context passed to every command.
type RunContext struct {
	Ctx          context.Context
	Client       *githubapi.Client
	Owner        string
	Repo         string
	SkipProfiles bool
	Now          time.Time
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
	return r
}

// Register adds a command. Use this to extend emberlens with custom commands.
func (r *Runner) Register(cmd Subcommand) {
	r.commands[cmd.Name()] = cmd
	r.order = append(r.order, cmd.Name())
}

func (r *Runner) helpText() string {
	var b strings.Builder
	b.WriteString("emberlens — GitHub repository people analytics from the CLI.\n\n")
	b.WriteString("Usage:\n  emberlens <command> [flags]\n\n")
	b.WriteString("Commands:\n")
	for _, name := range r.order {
		fmt.Fprintf(&b, "  %-24s %s\n", name, r.commands[name].Description())
	}
	b.WriteString(`
Global flags:
  -repo owner/repo         repository (required)
  -token <token>           GitHub token (or set GITHUB_TOKEN env var)
  -output table|json       output format (default: table)
  -verbose                 show all fields in detailed card layout
  -limit N                 show only top N results (default: 0 = all)
  -skip-profiles           skip fetching user profiles (saves API calls)
  -no-color                disable colored output
  -timeout <duration>      API timeout (default: 2m)
  -no-report               skip saving run report to disk
  -report-dir <dir>        report directory (default: emberlens-reports)

Use "emberlens <command> -h" for command-specific flags.
`)
	return b.String()
}

// Run parses arguments and executes the appropriate subcommand.
func (r *Runner) Run(args []string, envToken string) int {
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
	common := registerCommon(fs, envToken)
	cmd.RegisterFlags(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	owner, repo, err := parseRepo(common.repo)
	if err != nil {
		return r.fail(err)
	}

	// Setup
	dp := &display.Printer{W: r.Stdout, Color: !common.noColor}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), common.timeout)
	defer cancel()

	rc := &RunContext{
		Ctx:          ctx,
		Client:       githubapi.NewClient(common.token),
		Owner:        owner,
		Repo:         repo,
		SkipProfiles: common.skipProfiles,
		Now:          r.now(),
	}

	// Execute
	people, err := cmd.Execute(rc)
	if err != nil {
		return r.fail(err)
	}

	total := len(people)
	if common.limit > 0 && common.limit < len(people) {
		people = people[:common.limit]
	}
	elapsed := time.Since(start)

	// Render output
	if err := r.render(dp, people, common); err != nil {
		return r.fail(err)
	}

	// Save report
	var reportPath string
	if !common.noReport {
		cmdStr := reconstructCommand(cmd.Name(), fs)
		path, saveErr := report.Save(common.reportDir, cmd.Name(), common.repo, cmdStr, people, elapsed)
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
			lines = append(lines, fmt.Sprintf("Showing %d of %d results · %s", len(people), total, elapsed.Round(time.Millisecond)))
		} else {
			lines = append(lines, fmt.Sprintf("%d results · %s", total, elapsed.Round(time.Millisecond)))
		}
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
	repo         string
	token        string
	output       string
	verbose      bool
	limit        int
	skipProfiles bool
	noColor      bool
	timeout      time.Duration
	noReport     bool
	reportDir    string
}

func registerCommon(fs *flag.FlagSet, envToken string) *commonFlags {
	f := &commonFlags{}
	fs.StringVar(&f.repo, "repo", "", "repository in owner/repo format (required)")
	fs.StringVar(&f.token, "token", envToken, "GitHub token (defaults to GITHUB_TOKEN)")
	fs.StringVar(&f.output, "output", "table", "output format: table|json")
	fs.BoolVar(&f.verbose, "verbose", false, "show all fields in detailed card layout")
	fs.IntVar(&f.limit, "limit", 0, "show only top N results (0 = all)")
	fs.BoolVar(&f.skipProfiles, "skip-profiles", false, "skip fetching user profiles (saves API calls)")
	fs.BoolVar(&f.noColor, "no-color", false, "disable colored output")
	fs.DurationVar(&f.timeout, "timeout", 2*time.Minute, "API timeout duration")
	fs.BoolVar(&f.noReport, "no-report", false, "skip saving run report to disk")
	fs.StringVar(&f.reportDir, "report-dir", "emberlens-reports", "report directory")
	return f
}

// ---------------------------------------------------------------------------
// Output rendering
// ---------------------------------------------------------------------------

func (r *Runner) render(dp *display.Printer, people []analysis.Person, common *commonFlags) error {
	switch common.output {
	case "table":
		dp.Banner("emberlens", common.repo)
		if common.verbose {
			r.printCards(dp, people)
		} else {
			r.printTable(dp, people)
		}
		return nil
	case "json":
		enc := json.NewEncoder(r.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(people)
	default:
		return fmt.Errorf("unsupported -output=%q (expected table|json)", common.output)
	}
}

func (r *Runner) printTable(dp *display.Printer, people []analysis.Person) {
	headers := []string{"#", "LOGIN", "NAME", "CONTRIBUTIONS", "PROFILE"}
	rows := make([][]string, len(people))
	for i, p := range people {
		rows[i] = []string{
			fmt.Sprintf("%d", i+1),
			p.Login,
			emptyDash(p.Name),
			fmt.Sprintf("%d", p.Contributions),
			p.ProfileURL,
		}
	}
	dp.Table(headers, rows)
}

func (r *Runner) printCards(dp *display.Printer, people []analysis.Person) {
	for i, p := range people {
		dp.Card(i+1, []display.CardField{
			{Label: "Login", Value: p.Login},
			{Label: "Name", Value: emptyDash(p.Name)},
			{Label: "Contributions", Value: fmt.Sprintf("%d", p.Contributions)},
			{Label: "Profile", Value: p.ProfileURL},
			{Label: "Links", Value: emptyDash(strings.Join(p.ExternalLinks, ", "))},
			{Label: "Signals", Value: emptyDash(strings.Join(p.Signals, "; "))},
			{Label: "Reasons", Value: emptyDash(strings.Join(p.Reasons, " | "))},
		})
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
