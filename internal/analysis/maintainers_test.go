package analysis

import (
	"testing"

	"github.com/example/find-maintainers/internal/githubapi"
)

func TestBuildCandidates(t *testing.T) {
	contributors := []githubapi.Contributor{
		{User: githubapi.User{Login: "alice"}, Contributions: 90},
		{User: githubapi.User{Login: "bob"}, Contributions: 15},
		{User: githubapi.User{Login: "carol"}, Contributions: 3},
	}
	signals := map[string][]string{
		"bob": {"Public organization member"},
	}
	profiles := map[string]githubapi.Profile{
		"alice": {Login: "alice", HTMLURL: "https://github.com/alice", Blog: "alice.dev"},
		"bob":   {Login: "bob", HTMLURL: "https://github.com/bob"},
	}

	got, err := BuildCandidates(contributors, signals, profiles, Config{MinContributions: 20, TopPercent: 0.05})
	if err != nil {
		t.Fatalf("BuildCandidates() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("BuildCandidates() maintainers = %d, want 2", len(got))
	}
	if got[0].Login != "alice" {
		t.Fatalf("top maintainer = %s, want alice", got[0].Login)
	}
	if got[1].Login != "bob" {
		t.Fatalf("second maintainer = %s, want bob", got[1].Login)
	}
}

func TestExtractLinks(t *testing.T) {
	p := githubapi.Profile{
		Blog:            "example.com",
		TwitterUsername: "example",
		Bio:             "maintainer at https://maintainer.dev and docs at http://docs.example.com",
	}
	links := extractLinks(p)
	if len(links) != 4 {
		t.Fatalf("extractLinks() len = %d, want 4: %v", len(links), links)
	}
}
