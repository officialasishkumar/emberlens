// Package platform defines the shared types and client interface
// for repository API providers (GitHub, GitLab, etc.).
package platform

import (
	"context"
	"time"
)

// Client is the abstraction for repository API providers.
type Client interface {
	// ProfileBaseURL returns the base web URL for user profiles
	// (e.g. "https://github.com" or "https://gitlab.com").
	ProfileBaseURL() string

	GetRepo(ctx context.Context, owner, repo string) (Repo, error)
	ListContributors(ctx context.Context, owner, repo string, maxPages int) ([]Contributor, error)
	ListPullRequests(ctx context.Context, owner, repo string, maxPages int) ([]PullRequest, error)
	ListIssues(ctx context.Context, owner, repo string, opts IssueListOptions) ([]Issue, error)
	ListIssueComments(ctx context.Context, owner, repo string, number int, maxPages int) ([]IssueComment, error)
	ListOrgMembers(ctx context.Context, org string) ([]User, error)
	ListCommitsSince(ctx context.Context, owner, repo string, since time.Time, maxPages int) ([]Commit, error)
	GetProfile(ctx context.Context, login string) (Profile, error)
}

// ---------------------------------------------------------------------------
// Shared data types — used by analysis, display, and reporting layers.
// ---------------------------------------------------------------------------

type Repo struct {
	Owner struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"owner"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

type User struct {
	Login   string `json:"login"`
	HTMLURL string `json:"html_url"`
	Type    string `json:"type"`
}

type Contributor struct {
	User
	Contributions int `json:"contributions"`
}

type PullRequest struct {
	User              User   `json:"user"`
	AuthorAssociation string `json:"author_association"`
}

type IssueListOptions struct {
	MaxPages  int
	State     string
	Sort      string
	Direction string
}

type Issue struct {
	User              User   `json:"user"`
	PullRequest       any    `json:"pull_request"`
	AuthorAssociation string `json:"author_association"`
	Number            int    `json:"number"`
	Title             string `json:"title"`
	State             string `json:"state"`
	HTMLURL           string `json:"html_url"`
	Comments          int    `json:"comments"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ClosedAt          *time.Time `json:"closed_at"`
}

type IssueComment struct {
	User              User      `json:"user"`
	AuthorAssociation string    `json:"author_association"`
	Body              string    `json:"body"`
	HTMLURL           string    `json:"html_url"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Commit struct {
	Author *User `json:"author"`
	Commit struct {
		Author struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type Profile struct {
	Login           string `json:"login"`
	Name            string `json:"name"`
	HTMLURL         string `json:"html_url"`
	Blog            string `json:"blog"`
	TwitterUsername string `json:"twitter_username"`
	Bio             string `json:"bio"`
}
