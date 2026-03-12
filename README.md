# emberlens

`emberlens` is a Go CLI for people analytics on GitHub and GitLab repositories.

It focuses on practical team intelligence from command line workflows:

- `contributors`: all-time contributor leaderboard
- `active-contributors`: contributors active in a configurable time window
- `maintainers`: likely maintainers based on all-time contribution strength + team signals

This is intentionally lightweight and inspired by the idea of repository analytics tools (e.g. Augur), but packaged as a single binary + GitHub/GitLab API calls.

## Platform Support

emberlens supports both **GitHub** and **GitLab** as backend platforms:

| Feature | GitHub | GitLab |
|---------|--------|--------|
| Contributors | ✅ | ✅ |
| Active contributors | ✅ | ✅ |
| Maintainers + team signals | ✅ | ✅ |
| Profiles | ✅ | ✅ |
| Self-hosted instances | ❌ (github.com only) | ✅ (`-gitlab-url`) |

Use the `-platform` flag to select the backend (default: `github`).

## API Tokens

### GitHub Token

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

### GitLab Token

A GitLab personal access token is optional but recommended for higher rate limits and access to private project data.

To create one:

1. Go to your GitLab instance → **Settings** → **Access Tokens** (e.g. [gitlab.com/-/user_settings/personal_access_tokens](https://gitlab.com/-/user_settings/personal_access_tokens)).
2. Give it a descriptive name (e.g. `emberlens`).
3. Select scopes: `read_api` (sufficient for read-only access).
4. Click **Create personal access token** and copy the value.

Set it as an environment variable:

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
```

Or pass it directly:

```bash
emberlens contributors -repo mygroup/myproject -platform gitlab -token glpat-xxxxxxxxxxxxxxxxxxxx
```

For self-hosted GitLab instances, set the `-gitlab-url` flag:

```bash
emberlens contributors -repo mygroup/myproject -platform gitlab -gitlab-url https://gitlab.example.com
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
| `-platform github\|gitlab` | `github` | Platform to use |
| `-token <token>` | `GITHUB_TOKEN` or `GITLAB_TOKEN` env | API personal access token |
| `-gitlab-url <url>` | `https://gitlab.com` | GitLab instance URL (only used with `-platform gitlab`) |
| `-output table\|json` | `table` | Output format |
| `-verbose` | off | Show all fields in detailed card layout |
| `-limit N` | `20` | Show only top N results (0 = all) |
| `-profiles` | off | Fetch full user profiles (extra API calls) |
| `-no-color` | off | Disable ANSI colored output |
| `-timeout <duration>` | `2m` | API timeout |
| `-no-report` | off | Skip saving run report to disk |
| `-report-dir <dir>` | `emberlens-reports` | Directory for run reports |

### Saving API calls

GitHub's public API allows **60 requests/hour** unauthenticated and **5,000/hour** with a token. emberlens is fast by default — profiles and signals are off, results are capped at 20, and only 3 pages of contributor data are fetched. Opt‑in to more data when needed:

- **`-profiles`** — Fetch `/users/<login>` for every person (names, bios, links). A repo with 300 contributors adds 300+ extra calls.
- **`-signals`** *(maintainers only)* — Enable PR/issue scanning for team signals (adds more pages of API calls).
- **`-limit 0`** — Show all results instead of the default top 20.
- **`-max-pages N`** — Increase pages of contributor/commit data beyond the default 3.

The default run is already lean — just pass `-repo`:

```bash
emberlens maintainers -repo keploy/keploy
```

For full detail:

```bash
emberlens maintainers -repo keploy/keploy -profiles -signals -limit 0 -max-pages 0
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
- `-max-pages` max contributor pages to fetch (default `3`, 100 per page)

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
- `-signals` enable team signal detection (extra API calls, off by default)
- `-max-pages` max contributor pages to fetch (default `3`, 0 = all)

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

- GitHub: Public API only exposes **public** org members.
- GitHub: For private org/team insights and better rate limits, use a token.
- GitHub: If GitHub cannot map commit authors to a login (e.g., unmatched email), those commits are skipped in active contributor counts.
- GitLab: Contributor logins are resolved from project members when possible; unmatched contributors use their git author name.
- GitLab: Team signals are derived from project member access levels (Owner, Maintainer, Developer).
- GitLab: Self-hosted instances are supported via `-gitlab-url`.

## GitLab Examples

### Contributors (all-time)

```bash
emberlens contributors -repo gnome/gnome-shell -platform gitlab
```

### Active contributors (time window)

```bash
emberlens active-contributors -repo gnome/gnome-shell -platform gitlab -since 720h
```

### Maintainers

```bash
emberlens maintainers -repo gnome/gnome-shell -platform gitlab
```

### Self-hosted GitLab

```bash
emberlens contributors -repo mygroup/myrepo -platform gitlab -gitlab-url https://gitlab.example.com
```

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
