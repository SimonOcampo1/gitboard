// Package gh fetches the board's data from GitHub, reusing the session of
// the gh CLI through go-gh: gitboard never logs in on its own.
package gh

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Limit is how many rows each view loads.
const Limit = 50

type Repo struct {
	NameWithOwner string
	Name          string
	Description   string
	Language      string
	URL           string
	Private       bool
	Stars         int
	PushedAt      time.Time
}

type PR struct {
	Repo           string
	Number         int
	Title          string
	URL            string
	Body           string
	Draft          bool
	ReviewDecision string // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED or ""
	Checks         string // SUCCESS, FAILURE, PENDING... or "" when there are none
	UpdatedAt      time.Time
}

type Issue struct {
	Repo      string
	Number    int
	Title     string
	URL       string
	Body      string
	Labels    []string
	UpdatedAt time.Time
}

type Commit struct {
	Repo    string
	SHA     string
	Message string
	URL     string
	Date    time.Time
}

type Client struct {
	gql   *api.GraphQLClient
	rest  *api.RESTClient
	Login string
}

func New() (*Client, error) {
	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, err
	}
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, err
	}
	var resp struct {
		Viewer struct{ Login string }
	}
	if err := gql.Do(`query { viewer { login } }`, nil, &resp); err != nil {
		return nil, err
	}
	return &Client{gql: gql, rest: rest, Login: resp.Viewer.Login}, nil
}

type Kind int

const (
	KindPRs Kind = iota
	KindReviews
	KindIssues
)

// SearchQuery is the GitHub search string behind the PRs, Reviews and
// Issues views. Reviews drops your own PRs: those belong to PRs.
func SearchQuery(k Kind, login string) string {
	switch k {
	case KindPRs:
		return "is:pr is:open archived:false author:" + login
	case KindReviews:
		return "is:pr is:open archived:false review-requested:" + login + " -author:" + login
	default:
		return "is:issue is:open archived:false assignee:" + login
	}
}

func (c *Client) Repos() ([]Repo, error) {
	q := fmt.Sprintf(`query { viewer { repositories(first: %d, ownerAffiliations: OWNER, orderBy: {field: PUSHED_AT, direction: DESC}) {
		nodes { nameWithOwner name description url isPrivate stargazerCount pushedAt primaryLanguage { name } } } } }`, Limit)
	var resp struct {
		Viewer struct {
			Repositories struct {
				Nodes []struct {
					NameWithOwner, Name, Description, URL string
					IsPrivate                             bool
					StargazerCount                        int
					PushedAt                              time.Time
					PrimaryLanguage                       *struct{ Name string }
				}
			}
		}
	}
	if err := c.gql.Do(q, nil, &resp); err != nil {
		return nil, err
	}
	var out []Repo
	for _, n := range resp.Viewer.Repositories.Nodes {
		r := Repo{NameWithOwner: n.NameWithOwner, Name: n.Name, Description: n.Description, URL: n.URL,
			Private: n.IsPrivate, Stars: n.StargazerCount, PushedAt: n.PushedAt}
		if n.PrimaryLanguage != nil {
			r.Language = n.PrimaryLanguage.Name
		}
		out = append(out, r)
	}
	return out, nil
}

// PRs serves both the PRs view (KindPRs) and the Reviews view (KindReviews).
func (c *Client) PRs(k Kind) ([]PR, error) {
	q := fmt.Sprintf(`query($q: String!) { search(query: $q, type: ISSUE, first: %d) { nodes { ... on PullRequest {
		number title url body isDraft reviewDecision updatedAt repository { nameWithOwner }
		commits(last: 1) { nodes { commit { statusCheckRollup { state } } } } } } } }`, Limit)
	var resp struct {
		Search struct {
			Nodes []struct {
				Number                       int
				Title, URL, Body             string
				IsDraft                      bool
				ReviewDecision               string
				UpdatedAt                    time.Time
				Repository                   struct{ NameWithOwner string }
				Commits                      struct {
					Nodes []struct {
						Commit struct {
							StatusCheckRollup *struct{ State string }
						}
					}
				}
			}
		}
	}
	if err := c.gql.Do(q, map[string]interface{}{"q": SearchQuery(k, c.Login)}, &resp); err != nil {
		return nil, err
	}
	var out []PR
	for _, n := range resp.Search.Nodes {
		p := PR{Repo: n.Repository.NameWithOwner, Number: n.Number, Title: n.Title, URL: n.URL, Body: n.Body,
			Draft: n.IsDraft, ReviewDecision: n.ReviewDecision, UpdatedAt: n.UpdatedAt}
		if len(n.Commits.Nodes) > 0 && n.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
			p.Checks = n.Commits.Nodes[0].Commit.StatusCheckRollup.State
		}
		out = append(out, p)
	}
	return out, nil
}

func (c *Client) Issues() ([]Issue, error) {
	q := fmt.Sprintf(`query($q: String!) { search(query: $q, type: ISSUE, first: %d) { nodes { ... on Issue {
		number title url body updatedAt repository { nameWithOwner } labels(first: 10) { nodes { name } } } } } }`, Limit)
	var resp struct {
		Search struct {
			Nodes []struct {
				Number           int
				Title, URL, Body string
				UpdatedAt        time.Time
				Repository       struct{ NameWithOwner string }
				Labels           struct{ Nodes []struct{ Name string } }
			}
		}
	}
	if err := c.gql.Do(q, map[string]interface{}{"q": SearchQuery(KindIssues, c.Login)}, &resp); err != nil {
		return nil, err
	}
	var out []Issue
	for _, n := range resp.Search.Nodes {
		i := Issue{Repo: n.Repository.NameWithOwner, Number: n.Number, Title: n.Title, URL: n.URL, Body: n.Body, UpdatedAt: n.UpdatedAt}
		for _, l := range n.Labels.Nodes {
			i.Labels = append(i.Labels, l.Name)
		}
		out = append(out, i)
	}
	return out, nil
}

// Commits uses the REST commit search, which only indexes each repo's
// default branch: a commit on a feature branch shows up once merged.
func (c *Client) Commits() ([]Commit, error) {
	var resp struct {
		Items []struct {
			SHA     string
			HTMLURL string `json:"html_url"`
			Commit  struct {
				Message   string
				Committer struct{ Date time.Time }
			}
			Repository struct {
				FullName string `json:"full_name"`
			}
		}
	}
	path := fmt.Sprintf("search/commits?q=%s&sort=committer-date&order=desc&per_page=%d", url.QueryEscape("author:"+c.Login), Limit)
	if err := c.rest.Get(path, &resp); err != nil {
		return nil, err
	}
	var out []Commit
	for _, it := range resp.Items {
		out = append(out, Commit{Repo: it.Repository.FullName, SHA: it.SHA, Message: it.Commit.Message,
			URL: it.HTMLURL, Date: it.Commit.Committer.Date})
	}
	return out, nil
}

// Readme returns the repo's README as markdown, or "" when it has none.
func (c *Client) Readme(repo string) (string, error) {
	var resp struct{ Content string }
	if err := c.rest.Get("repos/"+repo+"/readme", &resp); err != nil {
		if strings.Contains(err.Error(), "404") {
			return "", nil
		}
		return "", err
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
	return string(b), err
}

// CommitFiles lists the files a commit touched, as "status path" lines.
func (c *Client) CommitFiles(repo, sha string) ([]string, error) {
	var resp struct {
		Files []struct{ Filename, Status string }
	}
	if err := c.rest.Get("repos/"+repo+"/commits/"+sha, &resp); err != nil {
		return nil, err
	}
	var out []string
	for _, f := range resp.Files {
		out = append(out, f.Status+"  "+f.Filename)
	}
	return out, nil
}
