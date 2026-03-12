package githubapi

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
)

const baseURL = "https://api.github.com"

// Client is a minimal GitHub API client for repository people insights.
type Client struct {
	httpClient *http.Client
	token      string
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 25 * time.Second},
		token:      token,
	}
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, query map[string]string, out any) error {
	u, err := url.Parse(baseURL + endpoint)
	if err != nil {
		return fmt.Errorf("parse GitHub URL: %w", err)
	}

	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create GitHub request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send GitHub request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GitHub API %s %s failed: status=%d body=%q", method, endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode GitHub response: %w", err)
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

type Issue struct {
	User              User   `json:"user"`
	PullRequest       any    `json:"pull_request"`
	AuthorAssociation string `json:"author_association"`
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

func (c *Client) GetRepo(ctx context.Context, owner, repo string) (Repo, error) {
	var out Repo
	return out, c.doJSON(ctx, http.MethodGet, "/repos/"+owner+"/"+repo, nil, &out)
}

func (c *Client) ListContributors(ctx context.Context, owner, repo string) ([]Contributor, error) {
	return collectPaged[Contributor](ctx, c, "/repos/"+owner+"/"+repo+"/contributors", nil, 0)
}

func (c *Client) ListPullRequests(ctx context.Context, owner, repo string, maxPages int) ([]PullRequest, error) {
	q := map[string]string{"state": "all", "sort": "updated", "direction": "desc"}
	return collectPaged[PullRequest](ctx, c, "/repos/"+owner+"/"+repo+"/pulls", q, maxPages)
}

func (c *Client) ListIssues(ctx context.Context, owner, repo string, maxPages int) ([]Issue, error) {
	q := map[string]string{"state": "all", "sort": "updated", "direction": "desc"}
	return collectPaged[Issue](ctx, c, "/repos/"+owner+"/"+repo+"/issues", q, maxPages)
}

func (c *Client) ListPublicOrgMembers(ctx context.Context, org string) ([]User, error) {
	return collectPaged[User](ctx, c, "/orgs/"+org+"/public_members", nil, 0)
}

func (c *Client) ListCommitsSince(ctx context.Context, owner, repo string, since time.Time, maxPages int) ([]Commit, error) {
	q := map[string]string{"since": since.UTC().Format(time.RFC3339)}
	return collectPaged[Commit](ctx, c, "/repos/"+owner+"/"+repo+"/commits", q, maxPages)
}

func (c *Client) GetProfile(ctx context.Context, login string) (Profile, error) {
	var out Profile
	return out, c.doJSON(ctx, http.MethodGet, "/users/"+login, nil, &out)
}
