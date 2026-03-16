package gitlabapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/officialasishkumar/emberlens/internal/platform"
)

// newTestClient creates a Client pointing at the given test server.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c := NewClient("test-token", srv.URL)
	return c
}

// ---------------------------------------------------------------------------
// GetRepo
// ---------------------------------------------------------------------------

func TestGetRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Errorf("expected PRIVATE-TOKEN header, got %q", r.Header.Get("PRIVATE-TOKEN"))
		}
		json.NewEncoder(w).Encode(glProject{
			ID:     1,
			Name:   "myrepo",
			Path:   "myrepo",
			PathNS: "mygroup/myrepo",
			WebURL: "https://gitlab.example.com/mygroup/myrepo",
			Namespace: struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
				Name string `json:"name"`
			}{Path: "mygroup", Kind: "group", Name: "My Group"},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	repo, err := c.GetRepo(context.Background(), "mygroup", "myrepo")
	if err != nil {
		t.Fatalf("GetRepo() error = %v", err)
	}
	if repo.Owner.Login != "mygroup" {
		t.Errorf("Owner.Login = %q, want %q", repo.Owner.Login, "mygroup")
	}
	if repo.Owner.Type != "Organization" {
		t.Errorf("Owner.Type = %q, want %q", repo.Owner.Type, "Organization")
	}
	if repo.Name != "myrepo" {
		t.Errorf("Name = %q, want %q", repo.Name, "myrepo")
	}
	if repo.FullName != "mygroup/myrepo" {
		t.Errorf("FullName = %q, want %q", repo.FullName, "mygroup/myrepo")
	}
}

func TestGetRepoUserNamespace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(glProject{
			ID:     2,
			Name:   "myrepo",
			PathNS: "alice/myrepo",
			WebURL: "https://gitlab.com/alice/myrepo",
			Namespace: struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
				Name string `json:"name"`
			}{Path: "alice", Kind: "user", Name: "Alice"},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	repo, err := c.GetRepo(context.Background(), "alice", "myrepo")
	if err != nil {
		t.Fatalf("GetRepo() error = %v", err)
	}
	if repo.Owner.Type != "User" {
		t.Errorf("Owner.Type = %q, want %q", repo.Owner.Type, "User")
	}
}

// ---------------------------------------------------------------------------
// ListContributors
// ---------------------------------------------------------------------------

func TestListContributors(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		ep := r.URL.EscapedPath()
		if ep == "/api/v4/projects/owner%2Frepo/repository/contributors" {
			json.NewEncoder(w).Encode([]glContributor{
				{Name: "Alice", Email: "alice@example.com", Commits: 50},
				{Name: "Bob", Email: "bob@example.com", Commits: 30},
			})
			return
		}
		if ep == "/api/v4/projects/owner%2Frepo/members/all" {
			json.NewEncoder(w).Encode([]glMember{
				{ID: 1, Username: "alice", Name: "Alice", WebURL: "https://gitlab.com/alice", AccessLevel: 40},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	contributors, err := c.ListContributors(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("ListContributors() error = %v", err)
	}
	if len(contributors) != 2 {
		t.Fatalf("len = %d, want 2", len(contributors))
	}

	// Alice should be matched to her GitLab username
	if contributors[0].Login != "alice" {
		t.Errorf("contributors[0].Login = %q, want %q", contributors[0].Login, "alice")
	}
	if contributors[0].HTMLURL != "https://gitlab.com/alice" {
		t.Errorf("contributors[0].HTMLURL = %q, want %q", contributors[0].HTMLURL, "https://gitlab.com/alice")
	}
	if contributors[0].Contributions != 50 {
		t.Errorf("contributors[0].Contributions = %d, want 50", contributors[0].Contributions)
	}

	// Bob was not matched to a member, so uses git name
	if contributors[1].Login != "Bob" {
		t.Errorf("contributors[1].Login = %q, want %q", contributors[1].Login, "Bob")
	}
}

// ---------------------------------------------------------------------------
// ListCommitsSince
// ---------------------------------------------------------------------------

func TestListCommitsSince(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ep := r.URL.EscapedPath()
		if ep == "/api/v4/projects/owner%2Frepo/repository/commits" {
			json.NewEncoder(w).Encode([]glCommit{
				{ID: "abc", AuthorName: "Alice", AuthorEmail: "alice@example.com", AuthoredDate: "2026-03-01T00:00:00Z"},
				{ID: "def", AuthorName: "Bob", AuthorEmail: "bob@example.com", AuthoredDate: "2026-03-02T00:00:00Z"},
			})
			return
		}
		if ep == "/api/v4/projects/owner%2Frepo/members/all" {
			json.NewEncoder(w).Encode([]glMember{
				{ID: 1, Username: "alice", Name: "Alice", WebURL: "https://gitlab.com/alice"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	commits, err := c.ListCommitsSince(context.Background(), "owner", "repo", since, 1)
	if err != nil {
		t.Fatalf("ListCommitsSince() error = %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("len = %d, want 2", len(commits))
	}
	if commits[0].Author.Login != "alice" {
		t.Errorf("commits[0].Author.Login = %q, want %q", commits[0].Author.Login, "alice")
	}
	if commits[1].Author.Login != "Bob" {
		t.Errorf("commits[1].Author.Login = %q, want %q", commits[1].Author.Login, "Bob")
	}
}

// ---------------------------------------------------------------------------
// ListPullRequests (merge requests)
// ---------------------------------------------------------------------------

func TestListPullRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ep := r.URL.EscapedPath()
		if ep == "/api/v4/projects/owner%2Frepo/members/all" {
			json.NewEncoder(w).Encode([]glMember{
				{ID: 1, Username: "alice", Name: "Alice", AccessLevel: 50},
				{ID: 2, Username: "bob", Name: "Bob", AccessLevel: 30},
			})
			return
		}
		if ep == "/api/v4/projects/owner%2Frepo/merge_requests" {
			json.NewEncoder(w).Encode([]glMergeRequest{
				{IID: 1, Author: glUser{Username: "alice", WebURL: "https://gitlab.com/alice"}},
				{IID: 2, Author: glUser{Username: "bob", WebURL: "https://gitlab.com/bob"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	prs, err := c.ListPullRequests(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("ListPullRequests() error = %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("len = %d, want 2", len(prs))
	}

	// Alice is Owner (access_level 50)
	if prs[0].AuthorAssociation != "OWNER" {
		t.Errorf("prs[0].AuthorAssociation = %q, want %q", prs[0].AuthorAssociation, "OWNER")
	}
	// Bob is Developer (access_level 30)
	if prs[1].AuthorAssociation != "COLLABORATOR" {
		t.Errorf("prs[1].AuthorAssociation = %q, want %q", prs[1].AuthorAssociation, "COLLABORATOR")
	}
}

// ---------------------------------------------------------------------------
// ListIssues
// ---------------------------------------------------------------------------

func TestListIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ep := r.URL.EscapedPath()
		if ep == "/api/v4/projects/owner%2Frepo/members/all" {
			json.NewEncoder(w).Encode([]glMember{
				{ID: 1, Username: "alice", AccessLevel: 40},
			})
			return
		}
		if ep == "/api/v4/projects/owner%2Frepo/issues" {
			json.NewEncoder(w).Encode([]glIssue{
				{IID: 1, Title: "Oldest issue", State: "opened", WebURL: "https://gitlab.com/owner/repo/-/issues/1", UserNotesCount: 3, Author: glUser{Username: "alice"}},
				{IID: 2, Title: "Second issue", State: "opened", WebURL: "https://gitlab.com/owner/repo/-/issues/2", UserNotesCount: 1, Author: glUser{Username: "carol"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	issues, err := c.ListIssues(context.Background(), "owner", "repo", platform.IssueListOptions{MaxPages: 1})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("len = %d, want 2", len(issues))
	}
	// Alice is Maintainer (access_level 40)
	if issues[0].AuthorAssociation != "MEMBER" {
		t.Errorf("issues[0].AuthorAssociation = %q, want %q", issues[0].AuthorAssociation, "MEMBER")
	}
	// Carol is not a member
	if issues[1].AuthorAssociation != "" {
		t.Errorf("issues[1].AuthorAssociation = %q, want empty", issues[1].AuthorAssociation)
	}
	// GitLab issues never have PullRequest set
	if issues[0].PullRequest != nil {
		t.Error("issues[0].PullRequest should be nil")
	}
	if issues[0].Title != "Oldest issue" {
		t.Errorf("issues[0].Title = %q, want %q", issues[0].Title, "Oldest issue")
	}
	if issues[0].Comments != 3 {
		t.Errorf("issues[0].Comments = %d, want 3", issues[0].Comments)
	}
}

func TestListIssueComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ep := r.URL.EscapedPath()
		if ep == "/api/v4/projects/owner%2Frepo/members/all" {
			json.NewEncoder(w).Encode([]glMember{
				{ID: 1, Username: "maintainer", AccessLevel: 40},
			})
			return
		}
		if ep == "/api/v4/projects/owner%2Frepo/issues/42/notes" {
			json.NewEncoder(w).Encode([]glNote{
				{Body: "system note", System: true, Author: glUser{Username: "maintainer"}},
				{Body: "human note", Author: glUser{Username: "maintainer", WebURL: "https://gitlab.com/maintainer"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	comments, err := c.ListIssueComments(context.Background(), "owner", "repo", 42, 1)
	if err != nil {
		t.Fatalf("ListIssueComments() error = %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("len = %d, want 1", len(comments))
	}
	if comments[0].AuthorAssociation != "MEMBER" {
		t.Errorf("comments[0].AuthorAssociation = %q, want %q", comments[0].AuthorAssociation, "MEMBER")
	}
	if comments[0].Body != "human note" {
		t.Errorf("comments[0].Body = %q, want %q", comments[0].Body, "human note")
	}
}

// ---------------------------------------------------------------------------
// ListOrgMembers (group members)
// ---------------------------------------------------------------------------

func TestListOrgMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]glMember{
			{ID: 1, Username: "alice", Name: "Alice", WebURL: "https://gitlab.com/alice", AccessLevel: 50},
			{ID: 2, Username: "bob", Name: "Bob", WebURL: "https://gitlab.com/bob", AccessLevel: 40},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	members, err := c.ListOrgMembers(context.Background(), "mygroup")
	if err != nil {
		t.Fatalf("ListOrgMembers() error = %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("len = %d, want 2", len(members))
	}
	if members[0].Login != "alice" {
		t.Errorf("members[0].Login = %q, want %q", members[0].Login, "alice")
	}
}

// ---------------------------------------------------------------------------
// GetProfile
// ---------------------------------------------------------------------------

func TestGetProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") != "alice" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode([]glUserDetailed{
			{
				ID:              1,
				Username:        "alice",
				Name:            "Alice Smith",
				WebURL:          "https://gitlab.com/alice",
				Bio:             "Developer",
				Website:         "https://alice.dev",
				TwitterUsername: "alice_dev",
			},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	profile, err := c.GetProfile(context.Background(), "alice")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if profile.Login != "alice" {
		t.Errorf("Login = %q, want %q", profile.Login, "alice")
	}
	if profile.Name != "Alice Smith" {
		t.Errorf("Name = %q, want %q", profile.Name, "Alice Smith")
	}
	if profile.HTMLURL != "https://gitlab.com/alice" {
		t.Errorf("HTMLURL = %q, want %q", profile.HTMLURL, "https://gitlab.com/alice")
	}
	if profile.Blog != "https://alice.dev" {
		t.Errorf("Blog = %q, want %q", profile.Blog, "https://alice.dev")
	}
}

func TestGetProfileNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]glUserDetailed{})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetProfile(context.Background(), "unknown")
	if err == nil {
		t.Fatal("GetProfile() expected error for unknown user")
	}
}

// ---------------------------------------------------------------------------
// accessLevelToAssociation
// ---------------------------------------------------------------------------

func TestAccessLevelToAssociation(t *testing.T) {
	tests := []struct {
		level int
		want  string
	}{
		{50, "OWNER"},
		{40, "MEMBER"},
		{30, "COLLABORATOR"},
		{20, ""},
		{10, ""},
		{0, ""},
	}
	for _, tt := range tests {
		got := accessLevelToAssociation(tt.level)
		if got != tt.want {
			t.Errorf("accessLevelToAssociation(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ProfileBaseURL
// ---------------------------------------------------------------------------

func TestProfileBaseURL(t *testing.T) {
	c1 := NewClient("token", "")
	if c1.ProfileBaseURL() != "https://gitlab.com" {
		t.Errorf("default ProfileBaseURL = %q, want %q", c1.ProfileBaseURL(), "https://gitlab.com")
	}

	c2 := NewClient("token", "https://gitlab.example.com")
	if c2.ProfileBaseURL() != "https://gitlab.example.com" {
		t.Errorf("custom ProfileBaseURL = %q, want %q", c2.ProfileBaseURL(), "https://gitlab.example.com")
	}

	// Trailing slash should be trimmed
	c3 := NewClient("token", "https://gitlab.example.com/")
	if c3.ProfileBaseURL() != "https://gitlab.example.com" {
		t.Errorf("trailing slash ProfileBaseURL = %q, want %q", c3.ProfileBaseURL(), "https://gitlab.example.com")
	}
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"403 Forbidden"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetRepo(context.Background(), "owner", "repo")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}
