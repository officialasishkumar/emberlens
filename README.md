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

- `-repo owner/repo` (required)
- `-token <token>` (defaults to `GITHUB_TOKEN`)
- `-output table|json` (default: `table`)

## Commands

### 1) Contributors (all-time)

```bash
emberlens contributors -repo golang/go
```

This uses the GitHub `/contributors` API and ranks by total contributions.

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
