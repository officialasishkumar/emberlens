# repo-insights

`repo-insights` is a Go CLI for people analytics on GitHub repositories with **no database**.

It focuses on practical team intelligence from command line workflows:

- `contributors`: all-time contributor leaderboard
- `active-contributors`: contributors active in a configurable time window
- `maintainers`: likely maintainers based on all-time contribution strength + team signals

This is intentionally lightweight and inspired by the idea of repository analytics tools (e.g. Augur), but packaged as a single binary + GitHub API calls.

## Install

Prefer installing the command into your PATH:

```bash
go install ./cmd/repo-insights
```

Then run directly:

```bash
repo-insights help
```

(Alternative local build: `go build -o repo-insights ./cmd/repo-insights`.)

## Common flags

All subcommands support:

- `-repo owner/repo` (required)
- `-token <token>` (defaults to `GITHUB_TOKEN`)
- `-output table|json` (default: `table`)

## Commands

### 1) Contributors (all-time)

```bash
repo-insights contributors -repo golang/go
```

This uses the GitHub `/contributors` API and ranks by total contributions.

### 2) Active contributors (time window)

```bash
repo-insights active-contributors -repo golang/go -since 720h
```

Flags:
- `-since` duration (default `720h` = 30 days)
- `-commit-pages` max pages of commits to scan (default `5`, 100 commits/page)

This uses `/commits?since=...` and counts commit activity by GitHub author login.

### 3) Maintainers

```bash
repo-insights maintainers -repo golang/go
```

Flags:
- `-min-contributions` (default `25`)
- `-top-percent` (default `0.02`)
- `-signal-weight` (default `25`)
- `-signal-pages` (default `3`)

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
