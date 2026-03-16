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

	"github.com/officialasishkumar/emberlens/internal/platform"
)

const baseURL = "https://api.github.com"
const profileBase = "https://github.com"

// Client is a GitHub API client implementing platform.Client.
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

// ProfileBaseURL returns the GitHub web URL for user profiles.
func (c *Client) ProfileBaseURL() string { return profileBase }

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

func (c *Client) GetRepo(ctx context.Context, owner, repo string) (platform.Repo, error) {
	var out platform.Repo
	return out, c.doJSON(ctx, http.MethodGet, "/repos/"+owner+"/"+repo, nil, &out)
}

func (c *Client) ListContributors(ctx context.Context, owner, repo string, maxPages int) ([]platform.Contributor, error) {
	return collectPaged[platform.Contributor](ctx, c, "/repos/"+owner+"/"+repo+"/contributors", nil, maxPages)
}

func (c *Client) ListPullRequests(ctx context.Context, owner, repo string, maxPages int) ([]platform.PullRequest, error) {
	q := map[string]string{"state": "all", "sort": "updated", "direction": "desc"}
	return collectPaged[platform.PullRequest](ctx, c, "/repos/"+owner+"/"+repo+"/pulls", q, maxPages)
}

func (c *Client) ListIssues(ctx context.Context, owner, repo string, opts platform.IssueListOptions) ([]platform.Issue, error) {
	q := map[string]string{
		"state":     defaultIssueState(opts.State),
		"sort":      defaultIssueSort(opts.Sort),
		"direction": defaultIssueDirection(opts.Direction),
	}
	return collectPaged[platform.Issue](ctx, c, "/repos/"+owner+"/"+repo+"/issues", q, opts.MaxPages)
}

func (c *Client) ListIssueComments(ctx context.Context, owner, repo string, number int, maxPages int) ([]platform.IssueComment, error) {
	q := map[string]string{"sort": "created", "direction": "asc"}
	endpoint := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	return collectPaged[platform.IssueComment](ctx, c, endpoint, q, maxPages)
}

func (c *Client) ListOrgMembers(ctx context.Context, org string) ([]platform.User, error) {
	return collectPaged[platform.User](ctx, c, "/orgs/"+org+"/public_members", nil, 0)
}

func (c *Client) ListCommitsSince(ctx context.Context, owner, repo string, since time.Time, maxPages int) ([]platform.Commit, error) {
	q := map[string]string{"since": since.UTC().Format(time.RFC3339)}
	return collectPaged[platform.Commit](ctx, c, "/repos/"+owner+"/"+repo+"/commits", q, maxPages)
}

func (c *Client) GetProfile(ctx context.Context, login string) (platform.Profile, error) {
	var out platform.Profile
	return out, c.doJSON(ctx, http.MethodGet, "/users/"+login, nil, &out)
}

func defaultIssueState(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "open", "closed", "all":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "all"
	}
}

func defaultIssueSort(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "created", "updated", "comments":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "updated"
	}
}

func defaultIssueDirection(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "asc", "desc":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "desc"
	}
}
