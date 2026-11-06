package ui

import (
	"charm.land/bubbles/v2/table"
	"github.com/SimonOcampo1/gitboard/internal/gh"
)

// view describes one of the five tabs. The first column is the flexible
// one: it takes the space the others leave. When the terminal is narrow,
// columns are hidden in the order given by drop.
type view struct {
	name    string
	columns []table.Column
	drop    []int
	help    []hint
	load    func(*gh.Client) ([]item, error)
}

// hint is one key of the help line.
type hint struct{ key, label string }

const (
	vRepos = iota
	vPRs
	vCommits
	vReviews
	vIssues
)

var views = [5]view{
	vRepos: {"Repos", []table.Column{{Title: "Name"}, {Title: "Lang", Width: 12}, {Title: "Stars", Width: 6}, {Title: "Vis", Width: 5}, {Title: "Push", Width: 5}},
		[]int{2, 1, 3}, []hint{{"c", "clone"}, {"g", "lazygit"}, {"n", "new PR"}},
		func(c *gh.Client) ([]item, error) { r, err := c.Repos(); return repoItems(r, c.Login), err }},
	vPRs: {"PRs", []table.Column{{Title: "Title"}, {Title: "Repo", Width: 22}, {Title: "State", Width: 9}, {Title: "Checks", Width: 8}, {Title: "Upd", Width: 4}},
		[]int{3, 1, 2}, []hint{{"d", "diff"}, {"m", "merge"}},
		func(c *gh.Client) ([]item, error) { p, err := c.PRs(gh.KindPRs); return prItems(p, c.Login), err }},
	vCommits: {"Commits", []table.Column{{Title: "Message"}, {Title: "Repo", Width: 22}, {Title: "SHA", Width: 8}, {Title: "Age", Width: 4}},
		[]int{2, 1}, []hint{{"y", "copy SHA"}},
		func(c *gh.Client) ([]item, error) { cm, err := c.Commits(); return commitItems(cm, c.Login), err }},
	vReviews: {"Reviews", []table.Column{{Title: "Title"}, {Title: "Repo", Width: 22}, {Title: "State", Width: 9}, {Title: "Checks", Width: 8}, {Title: "Upd", Width: 4}},
		[]int{3, 1, 2}, []hint{{"d", "diff"}, {"a", "approve"}},
		func(c *gh.Client) ([]item, error) { p, err := c.PRs(gh.KindReviews); return prItems(p, c.Login), err }},
	vIssues: {"Issues", []table.Column{{Title: "Title"}, {Title: "Repo", Width: 22}, {Title: "Labels", Width: 16}, {Title: "Upd", Width: 4}},
		[]int{2, 1}, []hint{{"x", "close"}},
		func(c *gh.Client) ([]item, error) { i, err := c.Issues(); return issueItems(i, c.Login), err }},
}

// minFlex is the narrowest the flexible column may get before other
// columns start being hidden.
const minFlex = 16

// fitColumns picks the columns that fit in width and sizes the flexible
// one. It returns the columns to show and their indexes in cols. Each
// column also pays 2 cells of table padding.
func fitColumns(cols []table.Column, drop []int, width int) ([]table.Column, []int) {
	return fitColumnsKeeping(cols, drop, width, "")
}

// fitColumnsKeeping is fitColumns that never hides the column titled pinned.
func fitColumnsKeeping(cols []table.Column, drop []int, width int, pinned string) ([]table.Column, []int) {
	hidden := map[int]bool{}
	flex := func() int {
		used := 0
		for i, c := range cols {
			if !hidden[i] {
				used += c.Width + 2
			}
		}
		return width - used
	}
	for _, d := range drop {
		if flex() >= minFlex {
			break
		}
		if cols[d].Title == pinned {
			continue
		}
		hidden[d] = true
	}
	var out []table.Column
	var keep []int
	for i, c := range cols {
		if hidden[i] {
			continue
		}
		if i == 0 {
			c.Width = max(8, flex())
		}
		out = append(out, c)
		keep = append(keep, i)
	}
	return out, keep
}
