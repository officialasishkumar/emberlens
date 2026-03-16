package analysis

import (
	"sort"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/platform"
)

const (
	DiscoverSortAge        = "age"
	DiscoverSortUpdated    = "updated"
	DiscoverSortDiscussion = "discussion"
	DiscoverSortHeat       = "heat"
	DiscoverSortComments   = "comments"
)

func BuildDiscoverUntriagedDataset(issues []platform.Issue, commentsByIssue map[int][]platform.IssueComment, now time.Time, minAge time.Duration, sortBy string) Dataset {
	snapshots := issueSnapshots(filterIssues(issues, func(issue platform.Issue) bool {
		return !strings.EqualFold(issue.State, "closed")
	}), commentsByIssue)

	candidates := make([]IssueSnapshot, 0, len(snapshots))
	var totalAge time.Duration
	var oldestAge time.Duration
	for _, snapshot := range snapshots {
		if snapshot.FirstMaintainerResponseAt != nil {
			continue
		}
		age := now.Sub(snapshot.CreatedAt)
		if age < minAge {
			continue
		}
		totalAge += age
		if age > oldestAge {
			oldestAge = age
		}
		candidates = append(candidates, snapshot)
	}

	sortDiscoverUntriaged(candidates, now, sortBy)

	return Dataset{
		Title: "Untriaged issues",
		Summary: []Stat{
			{Label: "Candidates", Value: formatCount(len(candidates))},
			{Label: "Avg age", Value: formatDuration(averageDurationFromTotal(totalAge, len(candidates)), "days")},
			{Label: "Oldest age", Value: formatDuration(oldestAge, "days")},
		},
		Columns: []Column{
			{Key: "issue", Label: "ISSUE"},
			{Key: "age", Label: "AGE"},
			{Key: "last_updated", Label: "LAST UPDATE"},
			{Key: "participants", Label: "PARTICIPANTS"},
			{Key: "comments", Label: "COMMENTS"},
			{Key: "author", Label: "AUTHOR"},
			{Key: "title", Label: "TITLE"},
		},
		Records: discoverUntriagedRecords(candidates, now),
		Hints: []string{
			"Flags: -view untriaged -min-age 168h -comment-pages 2 -sort age|updated -limit 0",
		},
	}
}

func BuildDiscoverNeedsMaintainerDataset(issues []platform.Issue, commentsByIssue map[int][]platform.IssueComment, now time.Time, minAge time.Duration, minComments int, minParticipants int, sortBy string) Dataset {
	snapshots := issueSnapshots(filterIssues(issues, func(issue platform.Issue) bool {
		return !strings.EqualFold(issue.State, "closed")
	}), commentsByIssue)

	candidates := make([]IssueSnapshot, 0, len(snapshots))
	totalParticipants := 0
	totalComments := 0
	for _, snapshot := range snapshots {
		if snapshot.FirstMaintainerResponseAt != nil {
			continue
		}
		if now.Sub(snapshot.CreatedAt) < minAge {
			continue
		}
		if snapshot.Comments < minComments {
			continue
		}
		if snapshot.Participants < minParticipants {
			continue
		}
		totalParticipants += snapshot.Participants
		totalComments += snapshot.Comments
		candidates = append(candidates, snapshot)
	}

	sortDiscoverNeedsMaintainer(candidates, now, sortBy)

	return Dataset{
		Title: "Issues needing maintainer attention",
		Summary: []Stat{
			{Label: "Candidates", Value: formatCount(len(candidates))},
			{Label: "Avg participants", Value: formatCount(averageInt(totalParticipants, len(candidates)))},
			{Label: "Avg comments", Value: formatCount(averageInt(totalComments, len(candidates)))},
		},
		Columns: []Column{
			{Key: "issue", Label: "ISSUE"},
			{Key: "participants", Label: "PARTICIPANTS"},
			{Key: "comments", Label: "COMMENTS"},
			{Key: "age", Label: "AGE"},
			{Key: "last_updated", Label: "LAST UPDATE"},
			{Key: "author", Label: "AUTHOR"},
			{Key: "title", Label: "TITLE"},
		},
		Records: discoverNeedsMaintainerRecords(candidates, now),
		Hints: []string{
			"Flags: -view needs-maintainer -min-age 168h -min-comments 3 -min-participants 3 -comment-pages 2 -sort discussion|age|updated -limit 0",
		},
	}
}

func BuildDiscoverHotspotsDataset(issues []platform.Issue, commentsByIssue map[int][]platform.IssueComment, now time.Time, since time.Time, minComments int, minParticipants int, sortBy string) Dataset {
	snapshots := issueSnapshots(filterIssues(issues, func(issue platform.Issue) bool {
		return !strings.EqualFold(issue.State, "closed")
	}), commentsByIssue)

	candidates := make([]IssueSnapshot, 0, len(snapshots))
	maxParticipants := 0
	totalComments := 0
	for _, snapshot := range snapshots {
		if snapshot.UpdatedAt.Before(since) {
			continue
		}
		if snapshot.Comments < minComments {
			continue
		}
		if snapshot.Participants < minParticipants {
			continue
		}
		if snapshot.Participants > maxParticipants {
			maxParticipants = snapshot.Participants
		}
		totalComments += snapshot.Comments
		candidates = append(candidates, snapshot)
	}

	sortDiscoverHotspots(candidates, sortBy)

	return Dataset{
		Title: "Issue hotspots",
		Summary: []Stat{
			{Label: "Hotspots", Value: formatCount(len(candidates))},
			{Label: "Max participants", Value: formatCount(maxParticipants)},
			{Label: "Avg comments", Value: formatCount(averageInt(totalComments, len(candidates)))},
		},
		Columns: []Column{
			{Key: "issue", Label: "ISSUE"},
			{Key: "participants", Label: "PARTICIPANTS"},
			{Key: "comments", Label: "COMMENTS"},
			{Key: "last_updated", Label: "LAST UPDATE"},
			{Key: "maintainer", Label: "MAINTAINER"},
			{Key: "author", Label: "AUTHOR"},
			{Key: "title", Label: "TITLE"},
		},
		Records: discoverHotspotRecords(candidates),
		Hints: []string{
			"Flags: -view hotspots -since 336h -min-comments 5 -min-participants 3 -comment-pages 2 -sort heat|updated|comments -limit 0",
		},
	}
}

func sortDiscoverUntriaged(in []IssueSnapshot, now time.Time, sortBy string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case DiscoverSortUpdated:
		sort.Slice(in, func(i, j int) bool {
			if in[i].UpdatedAt.Equal(in[j].UpdatedAt) {
				return in[i].Number < in[j].Number
			}
			return in[i].UpdatedAt.Before(in[j].UpdatedAt)
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

func sortDiscoverNeedsMaintainer(in []IssueSnapshot, now time.Time, sortBy string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case DiscoverSortUpdated:
		sort.Slice(in, func(i, j int) bool {
			if in[i].UpdatedAt.Equal(in[j].UpdatedAt) {
				return in[i].Number < in[j].Number
			}
			return in[i].UpdatedAt.After(in[j].UpdatedAt)
		})
	case DiscoverSortAge:
		sort.Slice(in, func(i, j int) bool {
			left := now.Sub(in[i].CreatedAt)
			right := now.Sub(in[j].CreatedAt)
			if left == right {
				if in[i].Participants == in[j].Participants {
					return in[i].Number < in[j].Number
				}
				return in[i].Participants > in[j].Participants
			}
			return left > right
		})
	default:
		sort.Slice(in, func(i, j int) bool {
			if in[i].Participants == in[j].Participants {
				if in[i].Comments == in[j].Comments {
					left := now.Sub(in[i].CreatedAt)
					right := now.Sub(in[j].CreatedAt)
					if left == right {
						return in[i].Number < in[j].Number
					}
					return left > right
				}
				return in[i].Comments > in[j].Comments
			}
			return in[i].Participants > in[j].Participants
		})
	}
}

func sortDiscoverHotspots(in []IssueSnapshot, sortBy string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case DiscoverSortUpdated:
		sort.Slice(in, func(i, j int) bool {
			if in[i].UpdatedAt.Equal(in[j].UpdatedAt) {
				return in[i].Number < in[j].Number
			}
			return in[i].UpdatedAt.After(in[j].UpdatedAt)
		})
	case DiscoverSortComments:
		sort.Slice(in, func(i, j int) bool {
			if in[i].Comments == in[j].Comments {
				if in[i].Participants == in[j].Participants {
					return in[i].Number < in[j].Number
				}
				return in[i].Participants > in[j].Participants
			}
			return in[i].Comments > in[j].Comments
		})
	default:
		sort.Slice(in, func(i, j int) bool {
			if in[i].Participants == in[j].Participants {
				if in[i].Comments == in[j].Comments {
					if in[i].UpdatedAt.Equal(in[j].UpdatedAt) {
						return in[i].Number < in[j].Number
					}
					return in[i].UpdatedAt.After(in[j].UpdatedAt)
				}
				return in[i].Comments > in[j].Comments
			}
			return in[i].Participants > in[j].Participants
		})
	}
}

func discoverUntriagedRecords(in []IssueSnapshot, now time.Time) []map[string]any {
	records := make([]map[string]any, 0, len(in))
	for _, issue := range in {
		records = append(records, map[string]any{
			"issue":        formatIssueID(issue.Number),
			"age":          formatDuration(now.Sub(issue.CreatedAt), "days"),
			"last_updated": issue.UpdatedAt.Format(time.DateOnly),
			"participants": issue.Participants,
			"comments":     issue.Comments,
			"author":       issue.Author,
			"title":        issue.Title,
			"url":          issue.URL,
		})
	}
	return records
}

func discoverNeedsMaintainerRecords(in []IssueSnapshot, now time.Time) []map[string]any {
	records := make([]map[string]any, 0, len(in))
	for _, issue := range in {
		records = append(records, map[string]any{
			"issue":        formatIssueID(issue.Number),
			"participants": issue.Participants,
			"comments":     issue.Comments,
			"age":          formatDuration(now.Sub(issue.CreatedAt), "days"),
			"last_updated": issue.UpdatedAt.Format(time.DateOnly),
			"author":       issue.Author,
			"title":        issue.Title,
			"url":          issue.URL,
		})
	}
	return records
}

func discoverHotspotRecords(in []IssueSnapshot) []map[string]any {
	records := make([]map[string]any, 0, len(in))
	for _, issue := range in {
		maintainer := issue.FirstMaintainerResponder
		if maintainer == "" {
			maintainer = "-"
		}
		records = append(records, map[string]any{
			"issue":        formatIssueID(issue.Number),
			"participants": issue.Participants,
			"comments":     issue.Comments,
			"last_updated": issue.UpdatedAt.Format(time.DateOnly),
			"maintainer":   maintainer,
			"author":       issue.Author,
			"title":        issue.Title,
			"url":          issue.URL,
		})
	}
	return records
}
