package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/platform"
)

type IssueSnapshot struct {
	Number                    int
	Title                     string
	Author                    string
	State                     string
	URL                       string
	Comments                  int
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	ClosedAt                  *time.Time
	Participants              int
	FirstMaintainerResponder  string
	FirstMaintainerResponseAt *time.Time
}

func BuildIssuesNewDataset(issues []platform.Issue, since time.Time, period string) Dataset {
	points, total := buildIssueSeries(filterIssues(issues, func(issue platform.Issue) bool {
		return !issue.CreatedAt.IsZero() && !issue.CreatedAt.Before(since)
	}), period, func(issue platform.Issue) time.Time {
		return issue.CreatedAt
	})

	return Dataset{
		Title: "Issues opened",
		Summary: []Stat{
			{Label: "New issues", Value: formatCount(total)},
			{Label: "Buckets", Value: formatCount(len(points))},
			{Label: "Window", Value: formatWindow(since)},
		},
		Columns: []Column{
			{Key: "period", Label: "PERIOD"},
			{Key: "count", Label: "NEW ISSUES"},
		},
		Records: pointsToRecords(points),
		Hints: []string{
			"Flags: -since 168h -period day|week|month -limit 0 -output json",
		},
	}
}

func BuildIssuesActiveDataset(issues []platform.Issue, since time.Time, period string) Dataset {
	points, total := buildIssueSeries(filterIssues(issues, func(issue platform.Issue) bool {
		return !issue.UpdatedAt.IsZero() && !issue.UpdatedAt.Before(since)
	}), period, func(issue platform.Issue) time.Time {
		return issue.UpdatedAt
	})

	return Dataset{
		Title: "Issues active",
		Summary: []Stat{
			{Label: "Active issues", Value: formatCount(total)},
			{Label: "Buckets", Value: formatCount(len(points))},
			{Label: "Window", Value: formatWindow(since)},
		},
		Columns: []Column{
			{Key: "period", Label: "PERIOD"},
			{Key: "count", Label: "ACTIVE ISSUES"},
		},
		Records: pointsToRecords(points),
		Hints: []string{
			"Flags: -since 168h -period day|week|month -limit 0 -output json",
		},
	}
}

func BuildIssuesClosedDataset(issues []platform.Issue, since time.Time, period string, unit string) Dataset {
	filtered := filterIssues(issues, func(issue platform.Issue) bool {
		return issue.ClosedAt != nil && !issue.ClosedAt.IsZero() && !issue.ClosedAt.Before(since)
	})
	points, total := buildIssueSeries(filtered, period, func(issue platform.Issue) time.Time {
		if issue.ClosedAt == nil {
			return time.Time{}
		}
		return *issue.ClosedAt
	})

	durations := make([]time.Duration, 0, len(filtered))
	for _, issue := range filtered {
		if issue.ClosedAt == nil {
			continue
		}
		durations = append(durations, issue.ClosedAt.Sub(issue.CreatedAt))
	}

	return Dataset{
		Title: "Issues closed",
		Summary: []Stat{
			{Label: "Closed issues", Value: formatCount(total)},
			{Label: "Avg resolution", Value: formatDuration(averageDuration(durations), unit)},
			{Label: "Window", Value: formatWindow(since)},
		},
		Columns: []Column{
			{Key: "period", Label: "PERIOD"},
			{Key: "count", Label: "CLOSED ISSUES"},
		},
		Records: pointsToRecords(points),
		Hints: []string{
			"Flags: -since 168h -period day|week|month -unit days|hours -output json",
		},
	}
}

func BuildIssueBacklogDataset(issues []platform.Issue, now time.Time, staleFor time.Duration, sortBy string) Dataset {
	openIssues := issueSnapshots(filterIssues(issues, func(issue platform.Issue) bool {
		return !strings.EqualFold(issue.State, "closed")
	}), nil)
	sortIssueBacklog(openIssues, now, sortBy)

	staleCount := 0
	oldestAge := time.Duration(0)
	for _, issue := range openIssues {
		age := now.Sub(issue.CreatedAt)
		if age > oldestAge {
			oldestAge = age
		}
		if now.Sub(issue.UpdatedAt) >= staleFor {
			staleCount++
		}
	}

	return Dataset{
		Title: "Issue backlog",
		Summary: []Stat{
			{Label: "Open issues", Value: formatCount(len(openIssues))},
			{Label: "Stale issues", Value: formatCount(staleCount)},
			{Label: "Oldest age", Value: formatDuration(oldestAge, "days")},
		},
		Columns: []Column{
			{Key: "issue", Label: "ISSUE"},
			{Key: "age", Label: "AGE"},
			{Key: "last_updated", Label: "LAST UPDATE"},
			{Key: "comments", Label: "COMMENTS"},
			{Key: "author", Label: "AUTHOR"},
			{Key: "title", Label: "TITLE"},
		},
		Records: backlogRecords(openIssues, now),
		Hints: []string{
			"Flags: -stale-for 720h -sort age|updated|comments -limit 0 -output json",
		},
	}
}

func BuildIssueAgeDataset(issues []platform.Issue, now time.Time) Dataset {
	openIssues := filterIssues(issues, func(issue platform.Issue) bool {
		return !strings.EqualFold(issue.State, "closed")
	})
	buckets := []struct {
		label string
		max   time.Duration
	}{
		{label: "0-7d", max: 7 * 24 * time.Hour},
		{label: "8-30d", max: 30 * 24 * time.Hour},
		{label: "31-90d", max: 90 * 24 * time.Hour},
		{label: "91-180d", max: 180 * 24 * time.Hour},
		{label: "181d+", max: 0},
	}

	counts := make([]int, len(buckets))
	var totalAge time.Duration
	var oldest time.Duration
	for _, issue := range openIssues {
		age := now.Sub(issue.CreatedAt)
		totalAge += age
		if age > oldest {
			oldest = age
		}
		for i, bucket := range buckets {
			if bucket.max == 0 || age <= bucket.max {
				counts[i]++
				break
			}
		}
	}

	records := make([]map[string]any, 0, len(buckets))
	for i, bucket := range buckets {
		records = append(records, map[string]any{
			"bucket": bucket.label,
			"count":  counts[i],
		})
	}

	return Dataset{
		Title: "Issue age",
		Summary: []Stat{
			{Label: "Open issues", Value: formatCount(len(openIssues))},
			{Label: "Avg age", Value: formatDuration(averageDurationFromTotal(totalAge, len(openIssues)), "days")},
			{Label: "Oldest age", Value: formatDuration(oldest, "days")},
		},
		Columns: []Column{
			{Key: "bucket", Label: "AGE BUCKET"},
			{Key: "count", Label: "COUNT"},
		},
		Records: records,
		Hints: []string{
			"Flags: -limit 0 -output json",
		},
	}
}

func BuildIssueResolutionDataset(issues []platform.Issue, since time.Time, unit string, sortBy string) Dataset {
	closed := filterIssues(issues, func(issue platform.Issue) bool {
		return issue.ClosedAt != nil && !issue.ClosedAt.IsZero() && !issue.ClosedAt.Before(since)
	})
	rows := make([]issueDurationRow, 0, len(closed))
	durations := make([]time.Duration, 0, len(closed))
	for _, issue := range closed {
		if issue.ClosedAt == nil {
			continue
		}
		duration := issue.ClosedAt.Sub(issue.CreatedAt)
		durations = append(durations, duration)
		rows = append(rows, issueDurationRow{
			Issue:    formatIssueID(issue.Number),
			Title:    issue.Title,
			Author:   issue.User.Login,
			ClosedAt: issue.ClosedAt.Format(time.DateOnly),
			Duration: duration,
			Comments: issue.Comments,
		})
	}
	sortIssueDurationRows(rows, sortBy)

	return Dataset{
		Title: "Issue resolution duration",
		Summary: []Stat{
			{Label: "Closed issues", Value: formatCount(len(rows))},
			{Label: "Avg duration", Value: formatDuration(averageDuration(durations), unit)},
			{Label: "P50 duration", Value: formatDuration(percentileDuration(durations, 0.50), unit)},
		},
		Columns: []Column{
			{Key: "issue", Label: "ISSUE"},
			{Key: "duration", Label: "RESOLUTION"},
			{Key: "closed_at", Label: "CLOSED"},
			{Key: "comments", Label: "COMMENTS"},
			{Key: "author", Label: "AUTHOR"},
			{Key: "title", Label: "TITLE"},
		},
		Records: issueDurationRecords(rows, unit),
		Hints: []string{
			"Flags: -since 720h -unit days|hours -sort duration|closed -limit 0",
		},
	}
}

func BuildIssueResponseDataset(issues []platform.Issue, commentsByIssue map[int][]platform.IssueComment, since time.Time, unit string) Dataset {
	snapshots := issueSnapshots(filterIssues(issues, func(issue platform.Issue) bool {
		return !issue.CreatedAt.IsZero() && !issue.CreatedAt.Before(since)
	}), commentsByIssue)
	sort.Slice(snapshots, func(i, j int) bool {
		left := responseDuration(snapshots[i])
		right := responseDuration(snapshots[j])
		if left == nil && right == nil {
			return snapshots[i].Number < snapshots[j].Number
		}
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return *left > *right
	})

	durations := make([]time.Duration, 0, len(snapshots))
	noResponse := 0
	for _, snapshot := range snapshots {
		if duration := responseDuration(snapshot); duration != nil {
			durations = append(durations, *duration)
		} else {
			noResponse++
		}
	}

	return Dataset{
		Title: "Maintainer response duration",
		Summary: []Stat{
			{Label: "Issues in window", Value: formatCount(len(snapshots))},
			{Label: "Responded", Value: formatCount(len(durations))},
			{Label: "No response", Value: formatCount(noResponse)},
			{Label: "Avg response", Value: formatDuration(averageDuration(durations), unit)},
		},
		Columns: []Column{
			{Key: "issue", Label: "ISSUE"},
			{Key: "response", Label: "RESPONSE"},
			{Key: "responder", Label: "RESPONDER"},
			{Key: "comments", Label: "COMMENTS"},
			{Key: "author", Label: "AUTHOR"},
			{Key: "title", Label: "TITLE"},
		},
		Records: issueResponseRecords(snapshots, unit),
		Hints: []string{
			"Flags: -since 720h -comment-pages 2 -unit days|hours -limit 0",
		},
	}
}

func BuildIssueParticipantsDataset(issues []platform.Issue, commentsByIssue map[int][]platform.IssueComment, since time.Time) Dataset {
	snapshots := issueSnapshots(filterIssues(issues, func(issue platform.Issue) bool {
		return !issue.CreatedAt.IsZero() && !issue.CreatedAt.Before(since)
	}), commentsByIssue)
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Participants == snapshots[j].Participants {
			return snapshots[i].Comments > snapshots[j].Comments
		}
		return snapshots[i].Participants > snapshots[j].Participants
	})

	totalParticipants := 0
	maxParticipants := 0
	for _, snapshot := range snapshots {
		totalParticipants += snapshot.Participants
		if snapshot.Participants > maxParticipants {
			maxParticipants = snapshot.Participants
		}
	}

	return Dataset{
		Title: "Issue participation",
		Summary: []Stat{
			{Label: "Issues in window", Value: formatCount(len(snapshots))},
			{Label: "Avg participants", Value: formatCount(averageInt(totalParticipants, len(snapshots)))},
			{Label: "Max participants", Value: formatCount(maxParticipants)},
		},
		Columns: []Column{
			{Key: "issue", Label: "ISSUE"},
			{Key: "participants", Label: "PARTICIPANTS"},
			{Key: "comments", Label: "COMMENTS"},
			{Key: "author", Label: "AUTHOR"},
			{Key: "title", Label: "TITLE"},
		},
		Records: issueParticipantRecords(snapshots),
		Hints: []string{
			"Flags: -since 720h -comment-pages 2 -limit 0 -output json",
		},
	}
}

func BuildIssueAbandonedDataset(issues []platform.Issue, commentsByIssue map[int][]platform.IssueComment, now time.Time, staleFor time.Duration) Dataset {
	snapshots := issueSnapshots(filterIssues(issues, func(issue platform.Issue) bool {
		return !strings.EqualFold(issue.State, "closed")
	}), commentsByIssue)
	abandoned := make([]IssueSnapshot, 0, len(snapshots))
	noMaintainerResponse := 0
	var oldestInactive time.Duration
	for _, snapshot := range snapshots {
		inactiveFor := now.Sub(snapshot.UpdatedAt)
		if inactiveFor < staleFor {
			continue
		}
		abandoned = append(abandoned, snapshot)
		if snapshot.FirstMaintainerResponseAt == nil {
			noMaintainerResponse++
		}
		if inactiveFor > oldestInactive {
			oldestInactive = inactiveFor
		}
	}
	sort.Slice(abandoned, func(i, j int) bool {
		return abandoned[i].UpdatedAt.Before(abandoned[j].UpdatedAt)
	})

	return Dataset{
		Title: "Abandoned issues",
		Summary: []Stat{
			{Label: "Abandoned issues", Value: formatCount(len(abandoned))},
			{Label: "No maintainer response", Value: formatCount(noMaintainerResponse)},
			{Label: "Longest inactivity", Value: formatDuration(oldestInactive, "days")},
		},
		Columns: []Column{
			{Key: "issue", Label: "ISSUE"},
			{Key: "inactive", Label: "INACTIVE"},
			{Key: "updated", Label: "LAST UPDATE"},
			{Key: "response", Label: "MAINTAINER RESPONSE"},
			{Key: "comments", Label: "COMMENTS"},
			{Key: "title", Label: "TITLE"},
		},
		Records: abandonedIssueRecords(abandoned, now),
		Hints: []string{
			"Flags: -stale-for 720h -comment-pages 2 -limit 0 -output json",
		},
	}
}

func BuildIssueCountsDataset(issues []platform.Issue, since time.Time) Dataset {
	openCount := 0
	closedCount := 0
	newInWindow := 0
	closedInWindow := 0
	for _, issue := range issues {
		if strings.EqualFold(issue.State, "closed") {
			closedCount++
		} else {
			openCount++
		}
		if !issue.CreatedAt.IsZero() && !issue.CreatedAt.Before(since) {
			newInWindow++
		}
		if issue.ClosedAt != nil && !issue.ClosedAt.IsZero() && !issue.ClosedAt.Before(since) {
			closedInWindow++
		}
	}

	return Dataset{
		Title: "Issue counts",
		Summary: []Stat{
			{Label: "Open", Value: formatCount(openCount)},
			{Label: "Closed", Value: formatCount(closedCount)},
			{Label: "Window", Value: formatWindow(since)},
		},
		Columns: []Column{
			{Key: "metric", Label: "METRIC"},
			{Key: "value", Label: "VALUE"},
		},
		Records: []map[string]any{
			{"metric": "Open issues", "value": openCount},
			{"metric": "Closed issues", "value": closedCount},
			{"metric": "New issues in window", "value": newInWindow},
			{"metric": "Closed issues in window", "value": closedInWindow},
		},
		Hints: []string{
			"Flags: -since 720h -output json",
		},
	}
}

func issueSnapshots(issues []platform.Issue, commentsByIssue map[int][]platform.IssueComment) []IssueSnapshot {
	snapshots := make([]IssueSnapshot, 0, len(issues))
	for _, issue := range issues {
		participantSet := map[string]struct{}{}
		if issue.User.Login != "" {
			participantSet[issue.User.Login] = struct{}{}
		}

		var responder string
		var responseAt *time.Time
		for _, comment := range commentsByIssue[issue.Number] {
			if comment.User.Login != "" {
				participantSet[comment.User.Login] = struct{}{}
			}
			if comment.User.Login == issue.User.Login {
				continue
			}
			if !isMaintainerAssociation(comment.AuthorAssociation) {
				continue
			}
			commentTime := comment.CreatedAt
			if responseAt == nil || commentTime.Before(*responseAt) {
				ts := commentTime
				responseAt = &ts
				responder = comment.User.Login
			}
		}

		snapshots = append(snapshots, IssueSnapshot{
			Number:                    issue.Number,
			Title:                     issue.Title,
			Author:                    issue.User.Login,
			State:                     issue.State,
			URL:                       issue.HTMLURL,
			Comments:                  issue.Comments,
			CreatedAt:                 issue.CreatedAt,
			UpdatedAt:                 issue.UpdatedAt,
			ClosedAt:                  issue.ClosedAt,
			Participants:              len(participantSet),
			FirstMaintainerResponder:  responder,
			FirstMaintainerResponseAt: responseAt,
		})
	}
	return snapshots
}

func filterIssues(issues []platform.Issue, keep func(platform.Issue) bool) []platform.Issue {
	filtered := make([]platform.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		if keep(issue) {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

type issueSeriesPoint struct {
	Start time.Time
	Count int
}

func buildIssueSeries(issues []platform.Issue, period string, ts func(platform.Issue) time.Time) ([]issueSeriesPoint, int) {
	counts := map[time.Time]int{}
	total := 0
	for _, issue := range issues {
		at := ts(issue)
		if at.IsZero() {
			continue
		}
		start := bucketStart(at, period)
		counts[start]++
		total++
	}

	keys := make([]time.Time, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })

	points := make([]issueSeriesPoint, 0, len(keys))
	for _, key := range keys {
		points = append(points, issueSeriesPoint{Start: key, Count: counts[key]})
	}
	return points, total
}

func pointsToRecords(points []issueSeriesPoint) []map[string]any {
	records := make([]map[string]any, 0, len(points))
	for _, point := range points {
		records = append(records, map[string]any{
			"period": point.Start.Format(time.DateOnly),
			"count":  point.Count,
		})
	}
	return records
}

func bucketStart(t time.Time, period string) time.Time {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "week":
		weekday := int(t.UTC().Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(t.Year(), t.Month(), t.Day()-(weekday-1), 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
}

func formatWindow(since time.Time) string {
	return since.Format("2006-01-02")
}

func isMaintainerAssociation(v string) bool {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "OWNER", "MEMBER", "COLLABORATOR":
		return true
	default:
		return false
	}
}

func sortIssueBacklog(in []IssueSnapshot, now time.Time, sortBy string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "updated":
		sort.Slice(in, func(i, j int) bool {
			return in[i].UpdatedAt.Before(in[j].UpdatedAt)
		})
	case "comments":
		sort.Slice(in, func(i, j int) bool {
			if in[i].Comments == in[j].Comments {
				return in[i].Number < in[j].Number
			}
			return in[i].Comments > in[j].Comments
		})
	default:
		sort.Slice(in, func(i, j int) bool {
			left := now.Sub(in[i].CreatedAt)
			right := now.Sub(in[j].CreatedAt)
			if left == right {
				return in[i].Number < in[j].Number
			}
			return left > right
		})
	}
}

func backlogRecords(in []IssueSnapshot, now time.Time) []map[string]any {
	records := make([]map[string]any, 0, len(in))
	for _, issue := range in {
		records = append(records, map[string]any{
			"issue":        formatIssueID(issue.Number),
			"age":          formatDuration(now.Sub(issue.CreatedAt), "days"),
			"last_updated": issue.UpdatedAt.Format(time.DateOnly),
			"comments":     issue.Comments,
			"author":       issue.Author,
			"title":        issue.Title,
			"url":          issue.URL,
		})
	}
	return records
}

type issueDurationRow struct {
	Issue    string
	Title    string
	Author   string
	ClosedAt string
	Duration time.Duration
	Comments int
}

func sortIssueDurationRows(in []issueDurationRow, sortBy string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "closed":
		sort.Slice(in, func(i, j int) bool {
			return in[i].ClosedAt > in[j].ClosedAt
		})
	default:
		sort.Slice(in, func(i, j int) bool {
			if in[i].Duration == in[j].Duration {
				return in[i].Issue < in[j].Issue
			}
			return in[i].Duration > in[j].Duration
		})
	}
}

func issueDurationRecords(in []issueDurationRow, unit string) []map[string]any {
	records := make([]map[string]any, 0, len(in))
	for _, row := range in {
		records = append(records, map[string]any{
			"issue":     row.Issue,
			"duration":  formatDuration(row.Duration, unit),
			"closed_at": row.ClosedAt,
			"comments":  row.Comments,
			"author":    row.Author,
			"title":     row.Title,
		})
	}
	return records
}

func issueResponseRecords(in []IssueSnapshot, unit string) []map[string]any {
	records := make([]map[string]any, 0, len(in))
	for _, issue := range in {
		response := "-"
		if duration := responseDuration(issue); duration != nil {
			response = formatDuration(*duration, unit)
		}
		responder := issue.FirstMaintainerResponder
		if responder == "" {
			responder = "-"
		}
		records = append(records, map[string]any{
			"issue":     formatIssueID(issue.Number),
			"response":  response,
			"responder": responder,
			"comments":  issue.Comments,
			"author":    issue.Author,
			"title":     issue.Title,
			"url":       issue.URL,
		})
	}
	return records
}

func issueParticipantRecords(in []IssueSnapshot) []map[string]any {
	records := make([]map[string]any, 0, len(in))
	for _, issue := range in {
		records = append(records, map[string]any{
			"issue":        formatIssueID(issue.Number),
			"participants": issue.Participants,
			"comments":     issue.Comments,
			"author":       issue.Author,
			"title":        issue.Title,
			"url":          issue.URL,
		})
	}
	return records
}

func abandonedIssueRecords(in []IssueSnapshot, now time.Time) []map[string]any {
	records := make([]map[string]any, 0, len(in))
	for _, issue := range in {
		response := "-"
		if duration := responseDuration(issue); duration != nil {
			response = formatDuration(*duration, "days")
		}
		records = append(records, map[string]any{
			"issue":    formatIssueID(issue.Number),
			"inactive": formatDuration(now.Sub(issue.UpdatedAt), "days"),
			"updated":  issue.UpdatedAt.Format(time.DateOnly),
			"response": response,
			"comments": issue.Comments,
			"title":    issue.Title,
			"url":      issue.URL,
		})
	}
	return records
}

func responseDuration(issue IssueSnapshot) *time.Duration {
	if issue.FirstMaintainerResponseAt == nil {
		return nil
	}
	duration := issue.FirstMaintainerResponseAt.Sub(issue.CreatedAt)
	return &duration
}

func averageDuration(values []time.Duration) time.Duration {
	total := time.Duration(0)
	for _, value := range values {
		total += value
	}
	return averageDurationFromTotal(total, len(values))
}

func averageDurationFromTotal(total time.Duration, count int) time.Duration {
	if count == 0 {
		return 0
	}
	return time.Duration(int64(total) / int64(count))
}

func averageInt(total, count int) int {
	if count == 0 {
		return 0
	}
	return total / count
}

func formatIssueID(number int) string {
	return fmt.Sprintf("#%d", number)
}
