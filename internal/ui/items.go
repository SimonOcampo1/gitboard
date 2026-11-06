package ui

import (
	"fmt"
	"sort"
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
	md     string                           // Sidebar markdown known up front
	fetch  func(*gh.Client) (string, error) // extra Sidebar markdown loaded on demand

	at      time.Time // what the newest/oldest orders sort by
	section string    // Repos view: "Personal" or "Collaborated"

	// Group rows: a blank spacer or a repo title between groups. They are
	// never selected; title is the repo, count its rows.
	group bool
	title string
	count int
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

// shortRepo drops the owner from repos of the signed-in account, so the
// Repo column spends its width on the name. Other owners stay visible.
func shortRepo(repo, login string) string {
	return strings.TrimPrefix(repo, login+"/")
}

func repoItems(rs []gh.Repo, login string) []item {
	var out []item
	for _, r := range rs {
		vis := "pub"
		if r.Private {
			vis = "priv"
		}
		repo := r.NameWithOwner
		section := "Collaborated"
		if r.Owned {
			section = "Personal"
		}
		out = append(out, item{
			cols: []string{shortRepo(repo, login), r.Language, fmt.Sprintf("★%d", r.Stars), vis, ago(r.PushedAt)}, at: r.PushedAt,
			url: r.URL, repo: repo, name: r.Name, section: section,
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

func prItems(ps []gh.PR, login string) []item {
	var out []item
	for _, p := range ps {
		checks := p.Checks
		if checks == "" {
			checks = "none"
		}
		md := fmt.Sprintf("# %s\n\n`%s#%d` · %s · checks: %s\n\n%s", p.Title, p.Repo, p.Number, prState(p), strings.ToLower(checks), p.Body)
		out = append(out, item{
			cols: []string{p.Title, fmt.Sprintf("%s#%d", shortRepo(p.Repo, login), p.Number), prState(p), strings.ToLower(checks), ago(p.UpdatedAt)}, at: p.UpdatedAt,
			url: p.URL, repo: p.Repo, number: p.Number, md: md,
		})
	}
	return out
}

func commitItems(cs []gh.Commit, login string) []item {
	var out []item
	for _, c := range cs {
		subject, _, _ := strings.Cut(c.Message, "\n")
		repo, sha := c.Repo, c.SHA
		out = append(out, item{
			cols: []string{subject, shortRepo(c.Repo, login), c.SHA[:7], ago(c.Date)}, at: c.Date,
			url: c.URL, repo: repo, sha: sha,
			md: fmt.Sprintf("`%s` · `%s`\n\n```\n%s\n```", repo, sha[:7], c.Message),
			fetch: func(cl *gh.Client) (string, error) {
				files, err := cl.CommitFiles(repo, sha)
				return "**Files**\n\n```\n" + strings.Join(files, "\n") + "\n```", err
			},
		})
	}
	return out
}

func issueItems(is []gh.Issue, login string) []item {
	var out []item
	for _, i := range is {
		labels := strings.Join(i.Labels, ", ")
		md := fmt.Sprintf("# %s\n\n`%s#%d`", i.Title, i.Repo, i.Number)
		if labels != "" {
			md += " · " + labels
		}
		out = append(out, item{
			cols: []string{i.Title, fmt.Sprintf("%s#%d", shortRepo(i.Repo, login), i.Number), labels, ago(i.UpdatedAt)}, at: i.UpdatedAt,
			url: i.URL, repo: i.Repo, number: i.Number, md: md + "\n\n" + i.Body,
		})
	}
	return out
}

// order is how a view sorts its rows; s cycles through them.
type order int

const (
	newestFirst order = iota
	oldestFirst
	aToZ
	zToA
	orderCount
)

func (o order) String() string {
	return [...]string{"newest first", "oldest first", "A→Z", "Z→A"}[o]
}

func byRepo(it item) string    { return it.repo }
func bySection(it item) string { return it.section }

// arrange is what a view shows: the rows of repo only (when set), whose
// text contains q, sorted by ord, and grouped under group rows by the key
// groupBy gives each row (no groups when nil).
func arrange(items []item, q, repo string, groupBy func(item) string, ord order) []item {
	var out []item
	for _, it := range filterItems(items, q) {
		if repo == "" || it.repo == repo {
			out = append(out, it)
		}
	}
	text := func(it item) string { return strings.ToLower(it.cols[0]) }
	sort.SliceStable(out, func(i, j int) bool {
		switch ord {
		case oldestFirst:
			return out[i].at.Before(out[j].at)
		case aToZ:
			return text(out[i]) < text(out[j])
		case zToA:
			return text(out[i]) > text(out[j])
		}
		return out[i].at.After(out[j].at)
	})
	if groupBy == nil {
		return out
	}
	// By date, a group sits where its first row does (newest first: the
	// most active group on top). Alphabetically, groups follow their key.
	first := map[string]int{}
	counts := map[string]int{}
	for i, it := range out {
		if _, ok := first[groupBy(it)]; !ok {
			first[groupBy(it)] = i
		}
		counts[groupBy(it)]++
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := groupBy(out[i]), groupBy(out[j])
		// Personal (Repos view) leads in every order: your own repos come first.
		if (a == "Personal") != (b == "Personal") {
			return a == "Personal"
		}
		switch ord {
		case aToZ:
			return strings.ToLower(a) < strings.ToLower(b)
		case zToA:
			return strings.ToLower(a) > strings.ToLower(b)
		}
		return first[a] < first[b]
	})
	var grouped []item
	for i, it := range out {
		if k := groupBy(it); i == 0 || groupBy(out[i-1]) != k {
			if i > 0 {
				grouped = append(grouped, item{group: true})
			}
			grouped = append(grouped, item{group: true, title: k, count: counts[k]})
		}
		grouped = append(grouped, it)
	}
	return grouped
}

// rowCount is how many real rows items holds, group rows aside.
func rowCount(items []item) int {
	n := 0
	for _, it := range items {
		if !it.group {
			n++
		}
	}
	return n
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
