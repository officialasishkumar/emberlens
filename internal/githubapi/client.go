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

// Client is a minimal GitHub API client for maintainer discovery.
type Client struct {
	httpClient *http.Client
	token      string
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		token:      token,
	}
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, query map[string]string, out any) (http.Header, error) {
	u, err := url.Parse(baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse github url: %w", err)
	}

	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("github API %s %s failed: status=%d body=%q", method, endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return nil, fmt.Errorf("decode github response: %w", err)
		}
	}
	return resp.Header, nil
}

func collectPaged[T any](ctx context.Context, c *Client, endpoint string, baseQuery map[string]string) ([]T, error) {
	query := make(map[string]string, len(baseQuery)+2)
	for k, v := range baseQuery {
		query[k] = v
	}
	if _, ok := query["per_page"]; !ok {
		query["per_page"] = "100"
	}

	var all []T
	for page := 1; ; page++ {
		query["page"] = strconv.Itoa(page)
		var batch []T
		_, err := c.doJSON(ctx, http.MethodGet, endpoint, query, &batch)
		if err != nil {
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
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
}

type User struct {
	Login     string `json:"login"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
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

type Profile struct {
	Login           string `json:"login"`
	Name            string `json:"name"`
	HTMLURL         string `json:"html_url"`
	Blog            string `json:"blog"`
	Company         string `json:"company"`
	Email           string `json:"email"`
	TwitterUsername string `json:"twitter_username"`
	Bio             string `json:"bio"`
}

func (c *Client) GetRepo(ctx context.Context, owner, repo string) (Repo, error) {
	var out Repo
	_, err := c.doJSON(ctx, http.MethodGet, "/repos/"+owner+"/"+repo, nil, &out)
	return out, err
}

func (c *Client) ListContributors(ctx context.Context, owner, repo string) ([]Contributor, error) {
	return collectPaged[Contributor](ctx, c, "/repos/"+owner+"/"+repo+"/contributors", nil)
}

func (c *Client) ListPullRequests(ctx context.Context, owner, repo string, maxPages int) ([]PullRequest, error) {
	return collectPagedLimited[PullRequest](ctx, c, "/repos/"+owner+"/"+repo+"/pulls", map[string]string{"state": "all", "sort": "updated", "direction": "desc"}, maxPages)
}

func (c *Client) ListIssues(ctx context.Context, owner, repo string, maxPages int) ([]Issue, error) {
	return collectPagedLimited[Issue](ctx, c, "/repos/"+owner+"/"+repo+"/issues", map[string]string{"state": "all", "sort": "updated", "direction": "desc"}, maxPages)
}

func collectPagedLimited[T any](ctx context.Context, c *Client, endpoint string, baseQuery map[string]string, maxPages int) ([]T, error) {
	query := make(map[string]string, len(baseQuery)+2)
	for k, v := range baseQuery {
		query[k] = v
	}
	query["per_page"] = "100"
	if maxPages <= 0 {
		maxPages = 1
	}

	var all []T
	for page := 1; page <= maxPages; page++ {
		query["page"] = strconv.Itoa(page)
		var batch []T
		_, err := c.doJSON(ctx, http.MethodGet, endpoint, query, &batch)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

func (c *Client) ListPublicOrgMembers(ctx context.Context, org string) ([]User, error) {
	return collectPaged[User](ctx, c, "/orgs/"+org+"/public_members", nil)
}

func (c *Client) GetProfile(ctx context.Context, login string) (Profile, error) {
	var out Profile
	_, err := c.doJSON(ctx, http.MethodGet, "/users/"+login, nil, &out)
	return out, err
}
