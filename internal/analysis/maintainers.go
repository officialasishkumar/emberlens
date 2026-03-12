package analysis

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/example/find-maintainers/internal/githubapi"
)

var urlRegex = regexp.MustCompile(`https?://[^\s)]+`)

type Candidate struct {
	Login             string   `json:"login"`
	Name              string   `json:"name,omitempty"`
	ProfileURL        string   `json:"profile_url"`
	Contributions     int      `json:"contributions"`
	TeamSignals       []string `json:"team_signals"`
	ExternalLinks     []string `json:"external_links"`
	MaintainerScore   int      `json:"maintainer_score"`
	MaintainerReasons []string `json:"maintainer_reasons"`
}

type Config struct {
	MinContributions int
	TopPercent       float64
}

type userState struct {
	contributions int
	signals       map[string]struct{}
	profile       githubapi.Profile
}

func BuildCandidates(contributors []githubapi.Contributor, teamSignals map[string][]string, profiles map[string]githubapi.Profile, cfg Config) ([]Candidate, error) {
	if cfg.MinContributions < 0 {
		return nil, fmt.Errorf("min contributions must be non-negative")
	}
	if cfg.TopPercent <= 0 || cfg.TopPercent > 1 {
		return nil, fmt.Errorf("top percent must be between 0 and 1")
	}

	state := map[string]*userState{}
	for _, c := range contributors {
		if c.Login == "" {
			continue
		}
		u := ensure(state, c.Login)
		u.contributions += c.Contributions
	}

	for login, signals := range teamSignals {
		u := ensure(state, login)
		for _, s := range signals {
			u.signals[s] = struct{}{}
		}
	}

	for login, p := range profiles {
		u := ensure(state, login)
		u.profile = p
	}

	totalContrib := 0
	for _, u := range state {
		totalContrib += u.contributions
	}
	minContribByPercent := int(float64(totalContrib) * cfg.TopPercent)
	if minContribByPercent < cfg.MinContributions {
		minContribByPercent = cfg.MinContributions
	}

	out := make([]Candidate, 0, len(state))
	for login, u := range state {
		signals := mapKeys(u.signals)
		sort.Strings(signals)

		score := u.contributions + len(signals)*25
		links := extractLinks(u.profile)
		reasons := make([]string, 0, 2)
		if u.contributions >= minContribByPercent {
			reasons = append(reasons, fmt.Sprintf("commit contributions (%d) meet threshold %d", u.contributions, minContribByPercent))
		}
		if len(signals) > 0 {
			reasons = append(reasons, "has team association signal(s): "+strings.Join(signals, ", "))
		}

		if len(reasons) == 0 {
			continue
		}

		out = append(out, Candidate{
			Login:             login,
			Name:              nonEmpty(u.profile.Name),
			ProfileURL:        nonEmptyOr(u.profile.HTMLURL, "https://github.com/"+login),
			Contributions:     u.contributions,
			TeamSignals:       signals,
			ExternalLinks:     links,
			MaintainerScore:   score,
			MaintainerReasons: reasons,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].MaintainerScore == out[j].MaintainerScore {
			return out[i].Login < out[j].Login
		}
		return out[i].MaintainerScore > out[j].MaintainerScore
	})

	return out, nil
}

func ensure(m map[string]*userState, login string) *userState {
	if _, ok := m[login]; !ok {
		m[login] = &userState{signals: map[string]struct{}{}}
	}
	return m[login]
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
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

	out := mapKeys(seen)
	sort.Strings(out)
	return out
}

func nonEmpty(v string) string {
	return strings.TrimSpace(v)
}

func nonEmptyOr(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}
