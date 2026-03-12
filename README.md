# emberlens

`emberlens` is a Go CLI for people analytics on GitHub repositories.

It focuses on practical team intelligence from command line workflows:

- `contributors`: all-time contributor leaderboard
- `active-contributors`: contributors active in a configurable time window
- `maintainers`: likely maintainers based on all-time contribution strength + team signals

This is intentionally lightweight and inspired by the idea of repository analytics tools (e.g. Augur), but packaged as a single binary + GitHub API calls.

## GitHub Token

A GitHub personal access token is optional but recommended for higher rate limits and access to private repository data.

To create one:

1. Go to [github.com/settings/tokens](https://github.com/settings/tokens).
2. Click **Generate new token** → **Generate new token (classic)**.
3. Give it a descriptive name (e.g. `emberlens`).
4. Select scopes: `repo` (for private repos) or leave all scopes unchecked (for public repos only).
5. Click **Generate token** and copy the value.

Set it as an environment variable:

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Or pass it directly:

```bash
emberlens contributors -repo golang/go -token ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## Install

Prefer installing the command into your PATH:

```bash
go install ./cmd/emberlens
```

Then run directly:

```bash
emberlens help
```

(Alternative local build: `go build -o emberlens ./cmd/emberlens`.)

## Common flags

All subcommands support:

| Flag | Default | Description |
|------|---------|-------------|
| `-repo owner/repo` | *(required)* | Target repository |
| `-token <token>` | `GITHUB_TOKEN` env | GitHub personal access token |
| `-output table\|json` | `table` | Output format |
| `-verbose` | off | Show all fields in detailed card layout |
| `-limit N` | `0` (all) | Show only top N results |
| `-skip-profiles` | off | Skip fetching user profiles (saves API calls) |
| `-no-color` | off | Disable ANSI colored output |
| `-timeout <duration>` | `2m` | API timeout |
| `-no-report` | off | Skip saving run report to disk |
| `-report-dir <dir>` | `emberlens-reports` | Directory for run reports |

### Saving API calls

GitHub's public API allows **60 requests/hour** unauthenticated and **5,000/hour** with a token. emberlens provides several flags to reduce API usage:

- **`-skip-profiles`** — Biggest saver. Skips fetching `/users/<login>` for every person. A repo with 300 contributors would normally make 300+ extra calls just for names/bios.
- **`-limit N`** — Only display top N results. Combined with `-skip-profiles`, this is very fast.
- **`-max-pages N`** — Cap how many pages of contributor/commit data to fetch.
- **`-skip-signals`** *(maintainers only)* — Skip PR/issue scanning for team signals.

Fastest possible run:

```bash
emberlens maintainers -repo keploy/keploy -skip-profiles -skip-signals -limit 20 -max-pages 3
```

### Display modes

**Default (compact table)** — shows `#`, `LOGIN`, `NAME`, `CONTRIBUTIONS`, `PROFILE`:

```bash
emberlens contributors -repo golang/go -limit 5
```

**Verbose (card layout)** — one card per person with all fields, ideal for small screens:

```bash
emberlens maintainers -repo keploy/keploy -verbose -limit 10
```

**JSON** — pipe to `jq` for custom filtering:

```bash
emberlens contributors -repo golang/go -output json | jq '.[0:5]'
```

## Commands

### 1) Contributors (all-time)

```bash
emberlens contributors -repo golang/go
```

This uses the GitHub `/contributors` API and ranks by total contributions.

Flags:
- `-max-pages` max contributor pages to fetch (default `10`, 100 per page)

### 2) Active contributors (time window)

```bash
emberlens active-contributors -repo golang/go -since 720h
```

Flags:
- `-since` duration (default `720h` = 30 days)
- `-commit-pages` max pages of commits to scan (default `5`, 100 commits/page)

This uses `/commits?since=...` and counts commit activity by GitHub author login.

### 3) Maintainers

```bash
emberlens maintainers -repo golang/go
```

Flags:
- `-min-contributions` minimum all-time contributions (default `25`)
- `-top-percent` top contribution share threshold (default `0.02`)
- `-signal-weight` score weight per team signal (default `25`)
- `-signal-pages` PR/issue pages for signal detection (default `3`)
- `-skip-signals` skip team signal detection entirely (saves API calls)
- `-max-pages` max contributor pages to fetch (default `0` = all)

Maintainer logic marks someone as likely maintainer if either:
- all-time contributions exceed threshold `max(min-contributions, top-percent * total repo contributions)`, or
- they have team signals (`OWNER`, `MEMBER`, `COLLABORATOR`, public org member)

Score for ranking:

```text
score = contributions + (signal-weight * team_signal_count)
```

## External links in output

When available, each person includes:
- GitHub profile URL
- blog URL
- Twitter/X URL
- URLs found in GitHub bio

## Notes

- Public API only exposes **public** org members.
- For private org/team insights and better rate limits, use a token.
- If GitHub cannot map commit authors to a login (e.g., unmatched email), those commits are skipped in active contributor counts.

## Reports

Every run automatically saves a YAML report to disk under `emberlens-reports/`. This helps with GitHub API rate limits — you can review previous results without re-fetching.

Reports are organized by subcommand with incrementing run numbers:

```
emberlens-reports/
  maintainers/
    run-0/
      report.yaml
    run-1/
      report.yaml
  contributors/
    run-0/
      report.yaml
  active-contributors/
    run-0/
      report.yaml
```

Each `report.yaml` contains:

```yaml
version: v1
name: maintainers-run-0
command: "emberlens maintainers -repo keploy/keploy"
repo: keploy/keploy
subcommand: maintainers
status: success
total: 42
created_at: "2026-03-12T10:30:00Z"
time_taken: "12.5s"
people:
  - login: user1
    name: User One
    # ... full person data
```

To skip report generation:

```bash
emberlens maintainers -repo keploy/keploy -no-report
```

To customize the report directory:

```bash
emberlens maintainers -repo keploy/keploy -report-dir ./my-reports
```

## Extending emberlens

emberlens uses a `Subcommand` interface for all commands. To add a new command:

1. Create a struct implementing the `Subcommand` interface in `internal/app/`:

```go
type myCmd struct {
    // command-specific flag fields
}

func (c *myCmd) Name() string                 { return "my-command" }
func (c *myCmd) Description() string           { return "Does something useful" }
func (c *myCmd) RegisterFlags(fs *flag.FlagSet) { /* add flags */ }
func (c *myCmd) Execute(rc *RunContext) ([]analysis.Person, error) {
    // use rc.Client, rc.Owner, rc.Repo, rc.Ctx
    // return []analysis.Person, nil
}
```

2. Register it in `NewRunner()` in `app.go`:

```go
r.Register(&myCmd{})
```

The runner handles flag parsing, output rendering (table/cards/JSON), report saving, and the summary footer automatically. Your command only needs to fetch data and return `[]Person`.
