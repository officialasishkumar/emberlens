# find-maintainers

`find-maintainers` is a Go CLI that identifies likely project maintainers for any GitHub repository.

It combines:
- **Historical contributions** (all-time commit contributions from `/contributors`)
- **Team affiliation signals** (GitHub `MEMBER`, `OWNER`, `COLLABORATOR`, and public org membership)
- **Public profile links** (blog, Twitter/X, and URLs in bio)

## Why this approach

Maintainers are usually people who either:
1. Have substantial long-term contribution volume, or
2. Are explicitly part of the project/team (owner/member/collaborator)

This tool flags both groups and ranks them by a score:

```text
score = commit_contributions + (25 * team_signal_count)
```

A person is considered a maintainer if they satisfy at least one:
- Contributions above threshold (`max(min-contributions, top-percent of total repo contributions)`), or
- At least one team-association signal.

## Install

```bash
go build -o bin/find-maintainers ./cmd/find-maintainers
```

## Usage

```bash
./bin/find-maintainers -repo golang/go
```

With JSON output:

```bash
./bin/find-maintainers -repo golang/go -output json
```

Using authenticated requests (recommended to avoid API limits):

```bash
GITHUB_TOKEN=... ./bin/find-maintainers -repo owner/repo
```

## Flags

- `-repo` (required): repository in `owner/repo` format
- `-token`: GitHub token (defaults to `GITHUB_TOKEN` env var)
- `-min-contributions` (default `25`): minimum all-time commit contributions
- `-top-percent` (default `0.02`): contribution share threshold as decimal (2% by default)
- `-signal-pages` (default `3`): number of pages (100 each) scanned from PRs/issues for team signals
- `-output` (`table|json`): output format

## Notes and limitations

- GitHub's public API only exposes **public org members**; private team membership requires permissions.
- Contributor stats are derived from GitHub's contributor endpoint and may exclude some edge cases (e.g., unmapped emails).
- For best accuracy, provide a token with enough permissions for higher rate limits.
