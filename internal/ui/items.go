package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/SimonOcampo1/gitboard/internal/gh"
)

// item is one row of any view, flattened so the table, the filter and the
// Sidebar can treat all five views the same way.
type item struct {
	cols   []string
	url    string
	repo   string // owner/name
	name   string // repo name only (Repos view)
	number int    // PR or issue number
	sha    string
	md     string                         // Sidebar markdown known up front
	fetch  func(*gh.Client) (string, error) // extra Sidebar markdown loaded on demand
}

// ago renders a timestamp the way gh-dash does: 5m, 3h, 2d, 4w, 1y.
func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/24/7))
	}
	return fmt.Sprintf("%dy", int(d.Hours()/24/365))
}

func repoItems(rs []gh.Repo) []item {
	var out []item
	for _, r := range rs {
		vis := "pub"
		if r.Private {
			vis = "priv"
		}
		repo := r.NameWithOwner
		out = append(out, item{
			cols: []string{r.Name, r.Language, fmt.Sprintf("★%d", r.Stars), vis, ago(r.PushedAt)},
			url:  r.URL, repo: repo, name: r.Name,
			fetch: func(c *gh.Client) (string, error) {
				md, err := c.Readme(repo)
				if err == nil && md == "" {
					md = "_No README._"
				}
				return md, err
			},
		})
	}
	return out
}

func prState(p gh.PR) string {
	switch {
	case p.Draft:
		return "draft"
	case p.ReviewDecision == "APPROVED":
		return "approved"
	case p.ReviewDecision == "CHANGES_REQUESTED":
		return "changes"
	}
	return "open"
}

func prItems(ps []gh.PR) []item {
	var out []item
	for _, p := range ps {
		checks := p.Checks
		if checks == "" {
			checks = "none"
		}
		md := fmt.Sprintf("# %s\n\n`%s#%d` · %s · checks: %s\n\n%s", p.Title, p.Repo, p.Number, prState(p), strings.ToLower(checks), p.Body)
		out = append(out, item{
			cols: []string{p.Title, fmt.Sprintf("%s#%d", p.Repo, p.Number), prState(p), strings.ToLower(checks), ago(p.UpdatedAt)},
			url:  p.URL, repo: p.Repo, number: p.Number, md: md,
		})
	}
	return out
}

func commitItems(cs []gh.Commit) []item {
	var out []item
	for _, c := range cs {
		subject, _, _ := strings.Cut(c.Message, "\n")
		repo, sha := c.Repo, c.SHA
		out = append(out, item{
			cols: []string{subject, c.Repo, c.SHA[:7], ago(c.Date)},
			url:  c.URL, repo: repo, sha: sha,
			md: fmt.Sprintf("`%s` · `%s`\n\n```\n%s\n```", repo, sha[:7], c.Message),
			fetch: func(cl *gh.Client) (string, error) {
				files, err := cl.CommitFiles(repo, sha)
				return "**Files**\n\n```\n" + strings.Join(files, "\n") + "\n```", err
			},
		})
	}
	return out
}

func issueItems(is []gh.Issue) []item {
	var out []item
	for _, i := range is {
		labels := strings.Join(i.Labels, ", ")
		md := fmt.Sprintf("# %s\n\n`%s#%d`", i.Title, i.Repo, i.Number)
		if labels != "" {
			md += " · " + labels
		}
		out = append(out, item{
			cols: []string{i.Title, fmt.Sprintf("%s#%d", i.Repo, i.Number), labels, ago(i.UpdatedAt)},
			url:  i.URL, repo: i.Repo, number: i.Number, md: md + "\n\n" + i.Body,
		})
	}
	return out
}

// filterItems keeps the rows whose visible text contains q, ignoring case.
func filterItems(items []item, q string) []item {
	if q == "" {
		return items
	}
	q = strings.ToLower(q)
	var out []item
	for _, it := range items {
		if strings.Contains(strings.ToLower(strings.Join(it.cols, " ")), q) {
			out = append(out, it)
		}
	}
	return out
}
