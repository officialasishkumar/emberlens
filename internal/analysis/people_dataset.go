package analysis

import "strings"

func ContributorsDataset(title string, people []Person, hints []string) Dataset {
	return Dataset{
		Title: title,
		Summary: []Stat{
			{Label: "Contributors", Value: formatCount(len(people))},
			{Label: "Top contribution count", Value: topContributionValue(people)},
		},
		Columns: []Column{
			{Key: "rank", Label: "#"},
			{Key: "login", Label: "LOGIN"},
			{Key: "name", Label: "NAME"},
			{Key: "contributions", Label: "CONTRIBUTIONS"},
			{Key: "profile_url", Label: "PROFILE"},
		},
		Records: peopleRecords(people),
		Hints:   hints,
	}
}

func MaintainersDataset(title string, people []Person, hints []string) Dataset {
	return Dataset{
		Title: title,
		Summary: []Stat{
			{Label: "Likely maintainers", Value: formatCount(len(people))},
			{Label: "Top score", Value: topScoreValue(people)},
		},
		Columns: []Column{
			{Key: "rank", Label: "#"},
			{Key: "login", Label: "LOGIN"},
			{Key: "name", Label: "NAME"},
			{Key: "score", Label: "SCORE"},
			{Key: "contributions", Label: "CONTRIBUTIONS"},
			{Key: "signals", Label: "SIGNALS"},
		},
		Records: peopleRecords(people),
		Hints:   hints,
	}
}

func peopleRecords(people []Person) []map[string]any {
	records := make([]map[string]any, 0, len(people))
	for i, person := range people {
		records = append(records, map[string]any{
			"rank":          i + 1,
			"login":         person.Login,
			"name":          emptyFallback(person.Name),
			"profile_url":   person.ProfileURL,
			"contributions": person.Contributions,
			"signals":       emptyFallback(strings.Join(person.Signals, "; ")),
			"links":         emptyFallback(strings.Join(person.ExternalLinks, ", ")),
			"score":         zeroAware(person.Score),
			"reasons":       emptyFallback(strings.Join(person.Reasons, " | ")),
		})
	}
	return records
}

func topContributionValue(people []Person) string {
	if len(people) == 0 {
		return "0"
	}
	return formatCount(people[0].Contributions)
}

func topScoreValue(people []Person) string {
	if len(people) == 0 {
		return "0"
	}
	return formatCount(people[0].Score)
}

func zeroAware(v int) any {
	if v == 0 {
		return "-"
	}
	return v
}

func emptyFallback(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
