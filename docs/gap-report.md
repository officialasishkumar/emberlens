# Emberlens Gap Report

Date: 2026-03-16

## Bottom line

Emberlens is a terminal-first analytics CLI.

It is intentionally narrower than full analytics platforms:

- live GitHub and GitLab API reads
- compact terminal output
- small, useful summary blocks
- flat run snapshots in YAML

If the goal is to expand analytics coverage, Emberlens should copy the metric surface of established platforms, not their operational CLI.

## What full analytics platforms provide

A typical repository analytics platform combines:

- collectors and worker processes
- PostgreSQL-backed historical storage
- derived and materialized metric tables
- REST and GraphQL endpoints
- dashboard and web-facing integrations
- some local analysis workflows for code, complexity, and ecosystem data

Its CLI is mainly for:

- backend lifecycle
- API lifecycle
- database initialization and migration
- repo and repo-group loading
- config management
- worker and cache control
- API key and user administration

So the honest answer to "what analytics can a platform CLI do directly?" is:

- very little directly
- most of the analytics live behind platform APIs, stored history, and dashboards

## What Emberlens provides now

Emberlens now covers three analytics domains well enough for a terminal product:

- people analytics
- issue analytics
- discovery analytics

It also has a generalized dataset model now, so the CLI is no longer locked to people-only rows.

### Product shape

Current Emberlens design:

- single binary CLI
- GitHub and GitLab support
- compact table output by default
- optional card view with `-verbose`
- optional JSON with `-output json`
- flat saved run reports under `test-run-N/`

### Global flags

Shared flags across commands:

- `-repo`
- `-platform`
- `-token`
- `-gitlab-url`
- `-output`
- `-verbose`
- `-limit`
- `-profiles`
- `-no-color`
- `-timeout`
- `-no-report`
- `-report-dir`

### Current analytics surface

#### People analytics

- `contributors`
- `active-contributors`
- `maintainers`

People-specific flags:

- `contributors`
  - `-max-pages`
- `active-contributors`
  - `-since`
  - `-commit-pages`
- `maintainers`
  - `-min-contributions`
  - `-top-percent`
  - `-signal-weight`
  - `-signal-pages`
  - `-signals`
  - `-max-pages`

#### Issue analytics

- `issues`
  - `-view counts`
  - `-view new`
  - `-view active`
  - `-view closed`
  - `-view backlog`
  - `-view age`
  - `-view resolution`
  - `-view response`
  - `-view participants`
  - `-view abandoned`

Issue-specific flags:

- time-series issue commands
  - `-since`
  - `-period`
  - `-max-pages`
- backlog and stale-work commands
  - `-stale-for`
  - `-sort`
  - `-comment-pages`
- duration and latency commands
  - `-unit`
  - `-comment-pages`

#### Discovery analytics

- `discover`
  - `-view untriaged`
  - `-view needs-maintainer`
  - `-view hotspots`

Discovery-specific flags:

- `-since`
- `-min-age`
- `-min-comments`
- `-min-participants`
- `-sort`
- `-comment-pages`
- `-max-pages`

### Terminal contract

The current terminal direction is correct and should stay consistent:

- default output must be small
- summary stats come first
- one compact table should answer the question fast
- flags reveal more detail when needed
- reports should mirror the terminal run instead of dumping everything

Current report layout follows that rule:

```text
emberlens-reports/
  test-run-0/
    report.yaml
  test-run-1/
    report.yaml
```

That is better than grouping report output by subcommand because the report is about a run, not a command family.

## What the new issue analytics cover

The newly added issue domain closes a large part of the original analytics gap.

Emberlens can now answer:

- new issue volume over time
- active issue volume over time
- closed issue volume over time
- open backlog shape
- open issue age distribution
- resolution duration for recently closed issues
- maintainer first-response duration
- per-issue participation counts
- abandoned issue detection
- open and closed issue counts

Important implementation notes:

- GitHub pull requests are excluded from issue metrics
- `issues -view active` uses the issue `updated_at` timestamp
- `issues -view response` uses first maintainer comment after issue creation
- GitHub maintainer inference uses author associations
- GitLab maintainer inference uses project member access levels
- abandoned issues are defined by inactivity since last update

This is a strong CLI-friendly subset of the issue analytics surface.

## Where Emberlens still has gaps

The remaining gap is no longer "issue analytics". It is the broader analytics surface that depends on richer platform data, stored history, or local repo analysis.

### Pull request and review analytics

Still missing in Emberlens:

- PR intake over time
- closed-without-merge counts
- acceptance rate
- review accepted and declined counts
- review duration
- time to close
- maintainer response latency on PRs
- merged-status distributions
- event-count and commit-count averages per PR

This is probably the next highest-value domain after issues.

### Release analytics

Still missing:

- releases
- tag-only releases
- release cadence

This should be relatively easy after expanding the platform client with release endpoints.

### Repo summary and metadata analytics

Still missing:

- stars
- forks
- watchers
- languages
- aggregate repo summary
- clone and traffic style metrics

This is important because broader analytics platforms give a wider repo-health picture than Emberlens does today.

### Commit and code analytics beyond people ranking

Still missing:

- committers over time
- first-time contributors
- top-committer concentration
- lines changed by author
- code churn summaries
- lines-of-code summaries
- complexity metrics
- file counts

Some of these fit API-only mode. Some need local clone analysis.

### Dependency, license, and SBOM analytics

Still missing:

- dependency inventory
- dependency freshness or libyear-style metrics
- license detection and coverage
- license counts
- SBOM export or inspection
- badge-style compliance indicators

These do not fit the current API-only architecture cleanly.

### Platform-level features Emberlens does not yet have

Still missing:

- historical storage
- repo groups
- multi-repo rollups
- batch querying
- API server
- dashboards
- collection completeness tracking
- insight-generation layers

These are platform features, not immediate CLI priorities.

## What Emberlens should not copy from full platforms

Emberlens should not try to recreate operational CLI groups of platform analytics tools unless the product direction changes completely.

Low-priority areas to avoid for now:

- backend start and stop commands
- DB bootstrap and migration commands
- cache and queue management commands
- app-user and API-key administration commands
- dashboard and web-service runtime commands

Those are infrastructure concerns, not user-facing terminal analytics.

## Missing parts inside Emberlens itself

Even with issue analytics added, there are still internal gaps that matter.

### 1. The platform client is still narrower than the full analytics metric surface

Current client methods cover:

- repo metadata
- contributors
- commits since a date
- pull requests
- issues
- issue comments
- org or group members
- user profiles

To support the next wave of analytics cleanly, Emberlens still needs platform methods for:

- releases
- repo language stats
- richer PR detail and review events
- repo metadata counts where exposed directly
- possibly diff and churn data

### 2. The output model is generalized, but not yet domain-rich

The dataset abstraction is now in place, which was a necessary LLD step.

Still useful next additions:

- richer field-selection support
- explicit sorting controls on more commands
- domain-specific summary schemas where needed
- optional CSV output if terminal users ask for spreadsheet export

### 3. There is still no historical storage layer

Without stored snapshots or cached history, Emberlens cannot reliably provide:

- long-range trend deltas across past runs
- historical stars, forks, or watchers
- clone traffic histories
- collection completeness
- repo-group rollups

This is one of the biggest structural differences between Emberlens and full analytics platforms.

### 4. There is still no local analysis layer

Without local repository scanning, Emberlens cannot add good versions of:

- LOC metrics
- complexity metrics
- dependency inventory
- license coverage
- SBOM generation

## Recommended next additions

These are the best next steps if Emberlens stays terminal-first.

### Priority 1: pull request and review analytics

Add:

- `prs-new`
- `prs-closed-no-merge`
- `pr-acceptance`
- `review-duration`
- `pr-response`
- `pr-time-to-close`

Recommended flags:

- `-since`
- `-period`
- `-unit`
- `-comment-pages` or review-page equivalent
- `-sort`

### Priority 2: repo summary metrics

Add:

- `repo-summary`
- `languages`
- `stars`
- `forks`
- `watchers`

Recommended flags:

- `-since` where relevant
- `-show`
- `-fields`
- `-limit`

### Priority 3: better contributor and commit analytics

Add:

- `contributors-new`
- `committers`
- `lines-changed-by-author`
- `code-changes`

Recommended flags:

- `-since`
- `-period`
- `-group-by`
- `-sort`

### Priority 4: local analysis only where the payoff is clear

Add later:

- LOC metrics
- complexity metrics
- dependency freshness
- license and SBOM workflows

## Recommended CLI design rules

To stay aligned with the terminal UX goal:

- default output should stay terse
- tables should answer one question, not every question
- flags should progressively reveal detail
- reports should store the run result, not an uncontrolled dump
- command growth should stay domain-based, not infrastructure-based

High-value flags that fit this model well:

- `-since`
- `-from`
- `-to`
- `-period`
- `-sort`
- `-order`
- `-fields`
- `-show`
- `-limit`
- `-all`
- `-explain`

## Final assessment

Emberlens is no longer just a people analytics prototype.

With the new issue analytics, it now covers one of the most useful analytics domains in a way that actually fits terminal workflows.

The main missing parts are now:

- pull request and review analytics
- releases
- repo summary metrics
- code and dependency analysis
- historical storage and multi-repo platform features

That is the right gap to have. It means Emberlens can keep moving forward as a strong CLI instead of becoming a partial clone of a backend analytics platform.

## Sources

Local Emberlens sources used:

- [README.md](/Users/asish/coding/projects/emberlens/README.md)
- [internal/app/app.go](/Users/asish/coding/projects/emberlens/internal/app/app.go)
- [internal/app/commands.go](/Users/asish/coding/projects/emberlens/internal/app/commands.go)
- [internal/app/issues.go](/Users/asish/coding/projects/emberlens/internal/app/issues.go)
- [internal/analysis/issues.go](/Users/asish/coding/projects/emberlens/internal/analysis/issues.go)
- [internal/platform/platform.go](/Users/asish/coding/projects/emberlens/internal/platform/platform.go)
- [internal/report/report.go](/Users/asish/coding/projects/emberlens/internal/report/report.go)
