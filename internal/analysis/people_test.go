package analysis

import (
	"testing"

	"github.com/officialasishkumar/emberlens/internal/platform"
)

func TestBuildMaintainers(t *testing.T) {
	contributors := []platform.Contributor{
		{User: platform.User{Login: "alice"}, Contributions: 100},
		{User: platform.User{Login: "bob"}, Contributions: 10},
	}
	signals := map[string][]string{
		"bob": {"Public org member"},
	}
	profiles := map[string]platform.Profile{
		"alice": {Name: "Alice", HTMLURL: "https://github.com/alice", Blog: "alice.dev"},
		"bob":   {Name: "Bob", HTMLURL: "https://github.com/bob"},
	}

	got, err := BuildMaintainers(contributors, signals, profiles, MaintainerConfig{MinContributions: 20, TopPercent: 0.05, SignalWeight: 25}, "https://github.com")
	if err != nil {
		t.Fatalf("BuildMaintainers() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("BuildMaintainers() len = %d, want 2", len(got))
	}
	if got[0].Login != "alice" {
		t.Fatalf("first login = %s, want alice", got[0].Login)
	}
}

func TestBuildActiveContributors(t *testing.T) {
	counts := map[string]int{"b": 2, "a": 5}
	got := BuildActiveContributors(counts, map[string]platform.Profile{}, "https://github.com")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Login != "a" {
		t.Fatalf("first login = %s, want a", got[0].Login)
	}
}

func TestExtractLinks(t *testing.T) {
	p := platform.Profile{
		Blog:            "example.com",
		TwitterUsername: "dev",
		Bio:             "docs https://docs.example.com",
	}
	links := extractLinks(p)
	if len(links) != 3 {
		t.Fatalf("len=%d links=%v", len(links), links)
	}
}
