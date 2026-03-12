// Package gitlabapi implements the platform.Client interface for the GitLab REST API.
package gitlabapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/officialasishkumar/emberlens/internal/platform"
)

const defaultBaseURL = "https://gitlab.com"

// Client is a GitLab API client implementing platform.Client.
type Client struct {
	httpClient *http.Client
	token      string
	apiURL     string // e.g. "https://gitlab.com/api/v4"
	webURL     string // e.g. "https://gitlab.com"
}

// NewClient creates a new GitLab client. baseURL is the GitLab instance URL
// (e.g. "https://gitlab.com" or "https://gitlab.example.com").
// Pass an empty string for the default gitlab.com.
func NewClient(token, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		httpClient: &http.Client{Timeout: 25 * time.Second},
		token:      token,
		apiURL:     baseURL + "/api/v4",
		webURL:     baseURL,
	}
}

// ProfileBaseURL returns the GitLab web URL for user profiles.
func (c *Client) ProfileBaseURL() string { return c.webURL }

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func (c *Client) doJSON(ctx context.Context, method, endpoint string, query map[string]string, out any) error {
	u, err := url.Parse(c.apiURL + endpoint)
	if err != nil {
		return fmt.Errorf("parse GitLab URL: %w", err)
	}

	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create GitLab request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send GitLab request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GitLab API %s %s failed: status=%d body=%q", method, endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode GitLab response: %w", err)
		}
	}
	return nil
}

func collectPaged[T any](ctx context.Context, c *Client, endpoint string, query map[string]string, maxPages int) ([]T, error) {
	q := make(map[string]string, len(query)+2)
	for k, v := range query {
		q[k] = v
	}
	q["per_page"] = "100"
	if maxPages <= 0 {
		maxPages = 1000
	}

	var all []T
	for page := 1; page <= maxPages; page++ {
		q["page"] = strconv.Itoa(page)
		var batch []T
		if err := c.doJSON(ctx, http.MethodGet, endpoint, q, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

func projectPath(owner, repo string) string {
	return url.PathEscape(owner + "/" + repo)
}

// ---------------------------------------------------------------------------
// GitLab-specific response types (internal)
// ---------------------------------------------------------------------------

type glProject struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	PathNS    string `json:"path_with_namespace"`
	WebURL    string `json:"web_url"`
	Namespace struct {
		Path string `json:"path"`
		Kind string `json:"kind"` // "group" or "user"
		Name string `json:"name"`
	} `json:"namespace"`
}

type glContributor struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Commits int    `json:"commits"`
}

type glMember struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	State       string `json:"state"`
	WebURL      string `json:"web_url"`
	AccessLevel int   `json:"access_level"`
}

type glUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	State    string `json:"state"`
	WebURL   string `json:"web_url"`
}

type glUserDetailed struct {
	ID              int    `json:"id"`
	Username        string `json:"username"`
	Name            string `json:"name"`
	WebURL          string `json:"web_url"`
	Bio             string `json:"bio"`
	Website         string `json:"website_url"`
	TwitterUsername string `json:"twitter"`
}

type glMergeRequest struct {
	IID    int    `json:"iid"`
	Author glUser `json:"author"`
}

type glIssue struct {
	IID    int    `json:"iid"`
	Author glUser `json:"author"`
}

type glCommit struct {
	ID           string `json:"id"`
	AuthorName   string `json:"author_name"`
	AuthorEmail  string `json:"author_email"`
	AuthoredDate string `json:"authored_date"`
}

// ---------------------------------------------------------------------------
// platform.Client implementation
// ---------------------------------------------------------------------------

func (c *Client) GetRepo(ctx context.Context, owner, repo string) (platform.Repo, error) {
	var gl glProject
	if err := c.doJSON(ctx, http.MethodGet, "/projects/"+projectPath(owner, repo), nil, &gl); err != nil {
		return platform.Repo{}, err
	}
	var r platform.Repo
	r.Owner.Login = gl.Namespace.Path
	if gl.Namespace.Kind == "group" {
		r.Owner.Type = "Organization"
	} else {
		r.Owner.Type = "User"
	}
	r.Name = gl.Name
	r.FullName = gl.PathNS
	r.HTMLURL = gl.WebURL
	return r, nil
}

func (c *Client) ListContributors(ctx context.Context, owner, repo string, maxPages int) ([]platform.Contributor, error) {
	pp := projectPath(owner, repo)
	q := map[string]string{"order_by": "commits", "sort": "desc"}
	glContribs, err := collectPaged[glContributor](ctx, c, "/projects/"+pp+"/repository/contributors", q, maxPages)
	if err != nil {
		return nil, err
	}

	// Fetch project members to map contributor names to GitLab usernames.
	members, _ := collectPaged[glMember](ctx, c, "/projects/"+pp+"/members/all", nil, 0)
	nameToMember := make(map[string]glMember, len(members))
	for _, m := range members {
		nameToMember[strings.ToLower(m.Name)] = m
	}

	out := make([]platform.Contributor, 0, len(glContribs))
	for _, gc := range glContribs {
		login := gc.Name
		htmlURL := ""
		if m, ok := nameToMember[strings.ToLower(gc.Name)]; ok {
			login = m.Username
			htmlURL = m.WebURL
		}
		out = append(out, platform.Contributor{
			User:          platform.User{Login: login, HTMLURL: htmlURL, Type: "User"},
			Contributions: gc.Commits,
		})
	}
	return out, nil
}

func (c *Client) ListPullRequests(ctx context.Context, owner, repo string, maxPages int) ([]platform.PullRequest, error) {
	pp := projectPath(owner, repo)

	// Get member access levels to populate AuthorAssociation.
	members, _ := collectPaged[glMember](ctx, c, "/projects/"+pp+"/members/all", nil, 0)
	accessMap := make(map[string]int, len(members))
	for _, m := range members {
		accessMap[m.Username] = m.AccessLevel
	}

	q := map[string]string{"state": "all", "order_by": "updated_at", "sort": "desc"}
	glMRs, err := collectPaged[glMergeRequest](ctx, c, "/projects/"+pp+"/merge_requests", q, maxPages)
	if err != nil {
		return nil, err
	}

	out := make([]platform.PullRequest, 0, len(glMRs))
	for _, mr := range glMRs {
		out = append(out, platform.PullRequest{
			User: platform.User{
				Login:   mr.Author.Username,
				HTMLURL: mr.Author.WebURL,
				Type:    "User",
			},
			AuthorAssociation: accessLevelToAssociation(accessMap[mr.Author.Username]),
		})
	}
	return out, nil
}

func (c *Client) ListIssues(ctx context.Context, owner, repo string, maxPages int) ([]platform.Issue, error) {
	pp := projectPath(owner, repo)

	members, _ := collectPaged[glMember](ctx, c, "/projects/"+pp+"/members/all", nil, 0)
	accessMap := make(map[string]int, len(members))
	for _, m := range members {
		accessMap[m.Username] = m.AccessLevel
	}

	q := map[string]string{"state": "all", "order_by": "updated_at", "sort": "desc"}
	glIssues, err := collectPaged[glIssue](ctx, c, "/projects/"+pp+"/issues", q, maxPages)
	if err != nil {
		return nil, err
	}

	out := make([]platform.Issue, 0, len(glIssues))
	for _, gi := range glIssues {
		out = append(out, platform.Issue{
			User: platform.User{
				Login:   gi.Author.Username,
				HTMLURL: gi.Author.WebURL,
				Type:    "User",
			},
			// GitLab issues never include merge requests, so PullRequest is always nil.
			PullRequest:       nil,
			AuthorAssociation: accessLevelToAssociation(accessMap[gi.Author.Username]),
		})
	}
	return out, nil
}

func (c *Client) ListOrgMembers(ctx context.Context, org string) ([]platform.User, error) {
	glMembers, err := collectPaged[glMember](ctx, c, "/groups/"+url.PathEscape(org)+"/members", nil, 0)
	if err != nil {
		return nil, err
	}
	out := make([]platform.User, 0, len(glMembers))
	for _, m := range glMembers {
		out = append(out, platform.User{
			Login:   m.Username,
			HTMLURL: m.WebURL,
			Type:    "User",
		})
	}
	return out, nil
}

func (c *Client) ListCommitsSince(ctx context.Context, owner, repo string, since time.Time, maxPages int) ([]platform.Commit, error) {
	pp := projectPath(owner, repo)
	q := map[string]string{"since": since.UTC().Format(time.RFC3339)}
	glCommits, err := collectPaged[glCommit](ctx, c, "/projects/"+pp+"/repository/commits", q, maxPages)
	if err != nil {
		return nil, err
	}

	// Fetch project members to resolve commit authors to GitLab usernames.
	members, _ := collectPaged[glMember](ctx, c, "/projects/"+pp+"/members/all", nil, 0)
	nameToMember := make(map[string]glMember, len(members))
	for _, m := range members {
		nameToMember[strings.ToLower(m.Name)] = m
	}

	out := make([]platform.Commit, 0, len(glCommits))
	for _, gc := range glCommits {
		login := gc.AuthorName
		if m, ok := nameToMember[strings.ToLower(gc.AuthorName)]; ok {
			login = m.Username
		}

		var pc platform.Commit
		pc.Author = &platform.User{Login: login}
		pc.Commit.Author.Name = gc.AuthorName
		pc.Commit.Author.Email = gc.AuthorEmail
		pc.Commit.Author.Date = gc.AuthoredDate
		out = append(out, pc)
	}
	return out, nil
}

func (c *Client) GetProfile(ctx context.Context, login string) (platform.Profile, error) {
	// GitLab user search by username.
	var users []glUserDetailed
	q := map[string]string{"username": login}
	if err := c.doJSON(ctx, http.MethodGet, "/users", q, &users); err != nil {
		return platform.Profile{}, err
	}
	if len(users) == 0 {
		return platform.Profile{}, fmt.Errorf("GitLab user %q not found", login)
	}
	u := users[0]
	return platform.Profile{
		Login:           u.Username,
		Name:            u.Name,
		HTMLURL:         u.WebURL,
		Blog:            u.Website,
		TwitterUsername: u.TwitterUsername,
		Bio:             u.Bio,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// accessLevelToAssociation maps GitLab access levels to platform-agnostic
// association strings compatible with GitHub's author_association.
//
//	50 = Owner → "OWNER"
//	40 = Maintainer → "MEMBER"
//	30 = Developer → "COLLABORATOR"
//	20 = Reporter, 10 = Guest → ""
func accessLevelToAssociation(level int) string {
	switch {
	case level >= 50:
		return "OWNER"
	case level >= 40:
		return "MEMBER"
	case level >= 30:
		return "COLLABORATOR"
	default:
		return ""
	}
}
