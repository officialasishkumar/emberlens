package analysis

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/officialasishkumar/emberlens/internal/githubapi"
)

var urlRegex = regexp.MustCompile(`https?://[^\s)]+`)

type Person struct {
	Login         string   `json:"login"`
	Name          string   `json:"name,omitempty"`
	ProfileURL    string   `json:"profile_url"`
	Contributions int      `json:"contributions"`
	Signals       []string `json:"signals,omitempty"`
	ExternalLinks []string `json:"external_links,omitempty"`
	Score         int      `json:"score,omitempty"`
	Reasons       []string `json:"reasons,omitempty"`
}

type MaintainerConfig struct {
	MinContributions int
	TopPercent       float64
	SignalWeight     int
}

func BuildContributors(contributors []githubapi.Contributor, profiles map[string]githubapi.Profile) []Person {
	people := make([]Person, 0, len(contributors))
	for _, c := range contributors {
		if strings.TrimSpace(c.Login) == "" {
			continue
		}
		people = append(people, personFrom(c.Login, c.Contributions, nil, profiles[c.Login]))
	}
	sortPeople(people)
	return people
}

func BuildActiveContributors(commitCounts map[string]int, profiles map[string]githubapi.Profile) []Person {
	people := make([]Person, 0, len(commitCounts))
	for login, count := range commitCounts {
		if strings.TrimSpace(login) == "" {
			continue
		}
		people = append(people, personFrom(login, count, nil, profiles[login]))
	}
	sortPeople(people)
	return people
}

func BuildMaintainers(contributors []githubapi.Contributor, teamSignals map[string][]string, profiles map[string]githubapi.Profile, cfg MaintainerConfig) ([]Person, error) {
	if cfg.MinContributions < 0 {
		return nil, fmt.Errorf("min contributions must be non-negative")
	}
	if cfg.TopPercent <= 0 || cfg.TopPercent > 1 {
		return nil, fmt.Errorf("top percent must be between 0 and 1")
	}
	if cfg.SignalWeight <= 0 {
		cfg.SignalWeight = 25
	}

	totals := map[string]int{}
	for _, c := range contributors {
		if strings.TrimSpace(c.Login) == "" {
			continue
		}
		totals[c.Login] += c.Contributions
	}

	totalRepoContrib := 0
	for _, v := range totals {
		totalRepoContrib += v
	}
	minThreshold := int(float64(totalRepoContrib) * cfg.TopPercent)
	if minThreshold < cfg.MinContributions {
		minThreshold = cfg.MinContributions
	}

	seen := map[string]struct{}{}
	for login := range totals {
		seen[login] = struct{}{}
	}
	for login := range teamSignals {
		seen[login] = struct{}{}
	}

	result := make([]Person, 0, len(seen))
	for login := range seen {
		signals := dedupeAndSort(teamSignals[login])
		contrib := totals[login]
		reasons := make([]string, 0, 2)
		if contrib >= minThreshold {
			reasons = append(reasons, fmt.Sprintf("all-time contributions %d >= threshold %d", contrib, minThreshold))
		}
		if len(signals) > 0 {
			reasons = append(reasons, "team signals: "+strings.Join(signals, ", "))
		}
		if len(reasons) == 0 {
			continue
		}

		p := personFrom(login, contrib, signals, profiles[login])
		p.Score = contrib + len(signals)*cfg.SignalWeight
		p.Reasons = reasons
		result = append(result, p)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].Login < result[j].Login
		}
		return result[i].Score > result[j].Score
	})

	return result, nil
}

func personFrom(login string, contributions int, signals []string, profile githubapi.Profile) Person {
	return Person{
		Login:         login,
		Name:          strings.TrimSpace(profile.Name),
		ProfileURL:    nonEmptyOr(strings.TrimSpace(profile.HTMLURL), "https://github.com/"+login),
		Contributions: contributions,
		Signals:       signals,
		ExternalLinks: extractLinks(profile),
	}
}

func dedupeAndSort(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	set := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			set[s] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func sortPeople(people []Person) {
	sort.Slice(people, func(i, j int) bool {
		if people[i].Contributions == people[j].Contributions {
			return people[i].Login < people[j].Login
		}
		return people[i].Contributions > people[j].Contributions
	})
}

func extractLinks(p githubapi.Profile) []string {
	seen := map[string]struct{}{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			v = "https://" + v
		}
		seen[v] = struct{}{}
	}

	add(p.Blog)
	if p.TwitterUsername != "" {
		add("https://twitter.com/" + p.TwitterUsername)
	}
	for _, m := range urlRegex.FindAllString(p.Bio, -1) {
		add(m)
	}

	out := make([]string, 0, len(seen))
	for link := range seen {
		out = append(out, link)
	}
	sort.Strings(out)
	return out
}

func nonEmptyOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
