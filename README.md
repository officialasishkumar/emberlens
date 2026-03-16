# emberlens

`emberlens` is a Go CLI for repository analytics on GitHub and GitLab.

It is designed for terminal-first usage:

- compact default output
- one summary block plus one useful table
- extra detail only when flags ask for it
- flat per-run YAML snapshots under `test-run-N/`

Today Emberlens covers two analytics domains:

- people analytics
- issue analytics

## What You Can Do

### People analytics

| Command | What it shows | Command flags |
|---|---|---|
| `contributors` | all-time contributor leaderboard | `-max-pages` |
| `active-contributors` | contributors active in a recent commit window | `-since`, `-commit-pages` |
| `maintainers` | likely maintainers from contribution weight and optional team signals | `-min-contributions`, `-top-percent`, `-signal-weight`, `-signal-pages`, `-signals`, `-max-pages` |

### Issue analytics

| Command | What it shows | Command flags |
|---|---|---|
| `issues` | issue analytics selected by `-view` | `-view`, `-since`, `-period`, `-unit`, `-sort`, `-stale-for`, `-comment-pages`, `-max-pages` |

| `-view` value | What it shows |
|---|---|
| `counts` | open and closed issue inventory plus recent totals |
| `new` | new issue volume over time |
| `active` | issue activity over time based on last update |
| `closed` | closed issue volume plus average resolution summary |
| `backlog` | oldest open issues in the backlog |
| `age` | open issue age distribution |
| `resolution` | resolution duration for recently closed issues |
| `response` | first maintainer response latency |
| `participants` | issues with the most distinct participants |
| `abandoned` | stale open issues with no recent activity |

## Platform Support

| Feature | GitHub | GitLab |
|---|---|---|
| People analytics | yes | yes |
| Issue analytics | yes | yes |
| User profiles | yes | yes |
| Self-hosted instances | GitHub.com only | yes via `-gitlab-url` |

Use `-platform github` or `-platform gitlab`. The default is `github`.

## Install

```bash
go install ./cmd/emberlens
```

Then:

```bash
emberlens help
```

For a local binary instead:

```bash
go build -o emberlens ./cmd/emberlens
```

## API Tokens

GitHub:

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

GitLab:

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
```

You can also pass a token directly:

```bash
emberlens contributors -repo golang/go -token "$GITHUB_TOKEN"
emberlens contributors -repo mygroup/myproject -platform gitlab -token "$GITLAB_TOKEN"
```

For self-hosted GitLab:

```bash
emberlens contributors -repo mygroup/myproject -platform gitlab -gitlab-url https://gitlab.example.com
```

## Output Model

Default table output is intentionally trimmed:

1. banner
2. summary stats
3. compact table
4. footer with elapsed time, truncation note, useful flags, and report path

Examples:

```bash
emberlens contributors -repo golang/go
emberlens issues -repo chaoss/augur -view new -since 720h -period week
emberlens issues -repo chaoss/augur -view backlog -stale-for 1440h
```

To reveal more detail:

```bash
emberlens issues -repo chaoss/augur -view resolution -verbose -limit 10
emberlens issues -repo chaoss/augur -view participants -output json | jq
emberlens maintainers -repo keploy/keploy -signals -profiles -limit 0
```

Key behavior:

- default row cap is `-limit 20`
- use `-limit 0` to show all rows
- `-verbose` switches from table view to card view
- `-output json` emits the dataset directly
- saved reports capture the same trimmed result shown by the run

## Common Flags

All commands support:

| Flag | Default | Description |
|---|---|---|
| `-repo owner/repo` | required | target repository |
| `-platform github|gitlab` | `github` | backend platform |
| `-token <token>` | env fallback | API token |
| `-gitlab-url <url>` | `https://gitlab.com` | GitLab base URL |
| `-output table|json` | `table` | output format |
| `-verbose` | off | detailed card layout |
| `-limit N` | `20` | max rows to render, `0` = all |
| `-profiles` | off | fetch full user profiles when supported |
| `-no-color` | off | disable ANSI color |
| `-timeout <duration>` | `2m` | request timeout |
| `-no-report` | off | skip YAML report output |
| `-report-dir <dir>` | `emberlens-reports` | report directory |

## Command Examples

### People

```bash
emberlens contributors -repo golang/go -max-pages 5
emberlens active-contributors -repo golang/go -since 168h -commit-pages 10
emberlens maintainers -repo keploy/keploy -signals -signal-pages 5
```

### Issues

```bash
emberlens issues -repo chaoss/augur
emberlens issues -repo chaoss/augur -view new -since 720h -period week
emberlens issues -repo chaoss/augur -view active -since 720h -period day
emberlens issues -repo chaoss/augur -view closed -since 720h -period week -unit days
emberlens issues -repo chaoss/augur -view backlog -stale-for 720h -sort age
emberlens issues -repo chaoss/augur -view age
emberlens issues -repo chaoss/augur -view resolution -since 1440h -unit days -sort duration
emberlens issues -repo chaoss/augur -view response -since 720h -comment-pages 2 -unit hours
emberlens issues -repo chaoss/augur -view participants -since 720h -comment-pages 2
emberlens issues -repo chaoss/augur -view abandoned -stale-for 720h -comment-pages 2
```

## Issue Analytics Notes

The issue commands are intentionally conservative and terminal-friendly:

- GitHub pull requests are excluded from issue analytics
- `issues -view active` uses the issue `updated_at` timestamp
- `issues -view response` measures first maintainer comment after issue creation
- GitHub maintainer response uses author associations like `OWNER`, `MEMBER`, and `COLLABORATOR`
- GitLab maintainer response is inferred from project member access levels
- `issues -view abandoned` uses inactivity since the last issue update

## Reports

Every run writes a YAML snapshot unless `-no-report` is set.

Reports are flat, not grouped by command:

```text
emberlens-reports/
  test-run-0/
    report.yaml
  test-run-1/
    report.yaml
```

Example report:

```yaml
version: v2
name: test-run-0
command: "emberlens issues -repo chaoss/augur -view=new -since=720h -period=week"
repo: chaoss/augur
status: success
total: 5
created_at: "2026-03-16T12:00:00Z"
time_taken: "842ms"
result:
  title: Issues opened
  summary:
    - label: New issues
      value: "29"
  columns:
    - key: period
      label: PERIOD
    - key: count
      label: NEW ISSUES
  records:
    - period: "2026-02-10"
      count: 4
```

Notes:

- reports mirror the rendered dataset after `-limit` is applied
- each run gets the next `test-run-N` directory
- use `-report-dir` to change the base folder

## Extending Emberlens

Every command implements the `Subcommand` interface and returns an `analysis.Dataset`.

```go
type myCmd struct{}

func (c *myCmd) Name() string        { return "my-command" }
func (c *myCmd) Description() string { return "Does something useful" }
func (c *myCmd) RegisterFlags(fs *flag.FlagSet) {}

func (c *myCmd) Execute(rc *RunContext) (analysis.Dataset, error) {
	return analysis.Dataset{
		Title: "My metric",
		Columns: []analysis.Column{
			{Key: "name", Label: "NAME"},
		},
		Records: []map[string]any{
			{"name": "example"},
		},
	}, nil
}
```

Register it in `NewRunner()`:

```go
r.Register(&myCmd{})
```

The runner handles:

- shared flag parsing
- output rendering
- card and table modes
- JSON output
- run reports
- summary footer

## Current Gaps Compared To Augur

Emberlens now covers people and issue analytics, but it still does not include major Augur domains:

- pull request and review analytics
- releases and cadence
- repo summary metrics like stars, forks, watchers, and languages
- code churn, lines-of-code, and complexity metrics
- dependency, license, and SBOM analysis
- historical storage, repo groups, and dashboard-style APIs

The detailed comparison is in [docs/augur-gap-report.md](/Users/asish/coding/projects/emberlens/docs/augur-gap-report.md).
