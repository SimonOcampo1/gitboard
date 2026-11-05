package ui

import (
	"charm.land/bubbles/v2/table"
	"github.com/SimonOcampo1/gitboard/internal/gh"
)

// view describes one of the five tabs. A column with Width 0 takes the
// space the others leave.
type view struct {
	name    string
	columns []table.Column
	help    string
	load    func(*gh.Client) ([]item, error)
}

const (
	vRepos = iota
	vPRs
	vCommits
	vReviews
	vIssues
)

var views = [5]view{
	vRepos: {"Repos", []table.Column{{Title: "Name"}, {Title: "Lang", Width: 12}, {Title: "Stars", Width: 6}, {Title: "Vis", Width: 5}, {Title: "Push", Width: 5}},
		"c clone · g lazygit · n new PR",
		func(c *gh.Client) ([]item, error) { r, err := c.Repos(); return repoItems(r), err }},
	vPRs: {"PRs", []table.Column{{Title: "Title"}, {Title: "Repo", Width: 22}, {Title: "State", Width: 9}, {Title: "Checks", Width: 8}, {Title: "Upd", Width: 4}},
		"d diff · m merge",
		func(c *gh.Client) ([]item, error) { p, err := c.PRs(gh.KindPRs); return prItems(p), err }},
	vCommits: {"Commits", []table.Column{{Title: "Message"}, {Title: "Repo", Width: 22}, {Title: "SHA", Width: 8}, {Title: "Age", Width: 4}},
		"y copy SHA",
		func(c *gh.Client) ([]item, error) { cm, err := c.Commits(); return commitItems(cm), err }},
	vReviews: {"Reviews", []table.Column{{Title: "Title"}, {Title: "Repo", Width: 22}, {Title: "State", Width: 9}, {Title: "Checks", Width: 8}, {Title: "Upd", Width: 4}},
		"d diff · a approve",
		func(c *gh.Client) ([]item, error) { p, err := c.PRs(gh.KindReviews); return prItems(p), err }},
	vIssues: {"Issues", []table.Column{{Title: "Title"}, {Title: "Repo", Width: 22}, {Title: "Labels", Width: 16}, {Title: "Upd", Width: 4}},
		"x close",
		func(c *gh.Client) ([]item, error) { i, err := c.Issues(); return issueItems(i), err }},
}

// fitColumns gives the flexible column whatever width is left. Each
// column also pays 2 cells of table padding.
func fitColumns(cols []table.Column, width int) []table.Column {
	out := make([]table.Column, len(cols))
	copy(out, cols)
	used := 0
	for _, c := range out {
		used += c.Width + 2
	}
	for i := range out {
		if out[i].Width == 0 {
			out[i].Width = max(10, width-used)
		}
	}
	return out
}
