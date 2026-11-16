package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/SimonOcampo1/gitboard/internal/gh"
)

func TestFilterByNumberIsExact(t *testing.T) {
	items := []item{{number: 42, cols: []string{"a", "#42"}}, {number: 420, cols: []string{"b", "#420"}}, {number: 142, cols: []string{"c", "#142"}}}
	if got := filterItems(items, "#42"); len(got) != 1 || got[0].number != 42 {
		t.Fatalf("filter #42 = %v", got)
	}
	// Without the #, it is a plain text search again.
	if got := filterItems(items, "42"); len(got) != 3 {
		t.Fatalf("filter 42 kept %d rows, want 3", len(got))
	}
}

func TestFilterItemsIgnoresCase(t *testing.T) {
	items := []item{{cols: []string{"gitboard", "Go"}}, {cols: []string{"ttt", "Lua"}}}
	if got := filterItems(items, "GO"); len(got) != 1 || got[0].cols[0] != "gitboard" {
		t.Fatalf("filter GO = %v", got)
	}
	if got := filterItems(items, ""); len(got) != 2 {
		t.Fatalf("empty filter should keep all, got %d", len(got))
	}
}

func TestFitColumnsGivesRestToFlexible(t *testing.T) {
	cols, keep := fitColumns([]table.Column{{Title: "Name"}, {Title: "Lang", Width: 10}}, []int{1}, 50)
	// 50 - (10+2) - (0+2) = 36
	if len(keep) != 2 || cols[0].Width != 36 || cols[1].Width != 10 {
		t.Fatalf("keep = %v, widths = %v", keep, cols)
	}
}

func TestFitColumnsHidesInDropOrder(t *testing.T) {
	all := []table.Column{{Title: "Name"}, {Title: "A", Width: 10}, {Title: "B", Width: 10}, {Title: "C", Width: 10}}
	// 40 cells: with all four the flexible column gets 40-38 = 2, too narrow.
	// Dropping C (first in drop) leaves 40-26 = 14, still under minFlex;
	// dropping A too leaves 26.
	cols, keep := fitColumns(all, []int{3, 1}, 40)
	if len(keep) != 2 || keep[0] != 0 || keep[1] != 2 || cols[0].Width != 26 {
		t.Fatalf("keep = %v, cols = %v", keep, cols)
	}
	// Nothing left to drop: the flexible column keeps an 8-cell floor.
	if cols, _ := fitColumns(all, nil, 10); cols[0].Width != 8 {
		t.Fatalf("flexible floor = %d, want 8", cols[0].Width)
	}
}

func TestArrangeFiltersAndGroupsByRepo(t *testing.T) {
	now := time.Now()
	items := []item{
		{repo: "o/b", cols: []string{"fix login"}, at: now},
		{repo: "o/a", cols: []string{"add tests"}, at: now.Add(-time.Hour)},
		{repo: "o/b", cols: []string{"add docs"}, at: now.Add(-2 * time.Hour)},
	}
	if got := arrange(items, "", "o/b", nil, newestFirst); len(got) != 2 || got[0].cols[0] != "fix login" {
		t.Fatalf("repo filter = %v", got)
	}
	// Grouped: o/b first (it has the newest row), rows keep their order
	// inside the group, a title row opens each group and a blank row
	// separates them.
	got := arrange(items, "", "", byRepo, newestFirst)
	want := []string{"title o/b 2", "fix login", "add docs", "spacer", "title o/a 1", "add tests"}
	if len(got) != len(want) {
		t.Fatalf("grouped = %d rows, want %d: %v", len(got), len(want), got)
	}
	for i, it := range got {
		var desc string
		switch {
		case it.group && it.title == "":
			desc = "spacer"
		case it.group:
			desc = fmt.Sprintf("title %s %d", it.title, it.count)
		default:
			desc = it.cols[0]
		}
		if desc != want[i] {
			t.Errorf("row %d = %q, want %q", i, desc, want[i])
		}
	}
	if n := rowCount(got); n != 3 {
		t.Errorf("rowCount = %d, want 3", n)
	}
	if got := arrange(items, "add", "o/b", nil, newestFirst); len(got) != 1 || got[0].cols[0] != "add docs" {
		t.Fatalf("text + repo filter = %v", got)
	}
}

func TestAgo(t *testing.T) {
	cases := map[time.Duration]string{
		5 * time.Minute:      "5m",
		3 * time.Hour:        "3h",
		2 * 24 * time.Hour:   "2d",
		21 * 24 * time.Hour:  "3w",
		400 * 24 * time.Hour: "1y",
	}
	for d, want := range cases {
		if got := ago(time.Now().Add(-d)); got != want {
			t.Errorf("ago(-%v) = %q, want %q", d, got, want)
		}
	}
}

func TestShortRepoKeepsOtherOwners(t *testing.T) {
	if got := shortRepo("sto/gitboard", "sto"); got != "gitboard" {
		t.Fatalf("own repo = %q", got)
	}
	if got := shortRepo("other/gitboard", "sto"); got != "other/gitboard" {
		t.Fatalf("other owner = %q", got)
	}
}

func TestFitColumnsKeepingNeverHidesKept(t *testing.T) {
	all := []table.Column{{Title: "Message"}, {Title: "Repo", Width: 22}, {Title: "SHA", Width: 8}}
	_, keep := fitColumnsKeeping(all, []int{2, 1}, 40, "Repo")
	if len(keep) != 2 || keep[1] != 1 {
		t.Fatalf("keep = %v, want Message and Repo", keep)
	}
}

func TestArrangeOrders(t *testing.T) {
	now := time.Now()
	items := []item{
		{repo: "o/b", cols: []string{"beta"}, at: now.Add(-time.Hour)},
		{repo: "o/a", cols: []string{"Alpha"}, at: now.Add(-2 * time.Hour)},
		{repo: "o/c", cols: []string{"gamma"}, at: now},
	}
	cases := map[order]string{
		newestFirst: "gamma beta Alpha",
		oldestFirst: "Alpha beta gamma",
		aToZ:        "Alpha beta gamma", // case does not matter
		zToA:        "gamma beta Alpha",
	}
	for ord, want := range cases {
		var got []string
		for _, it := range arrange(items, "", "", nil, ord) {
			got = append(got, it.cols[0])
		}
		if g := strings.Join(got, " "); g != want {
			t.Errorf("%v: got %q, want %q", ord, g, want)
		}
	}
	// Alphabetical orders put the groups in repo-name order.
	var titles []string
	for _, it := range arrange(items, "", "", byRepo, zToA) {
		if it.title != "" {
			titles = append(titles, it.title)
		}
	}
	if g := strings.Join(titles, " "); g != "o/c o/b o/a" {
		t.Errorf("Z→A groups = %q", g)
	}
}

func TestRepoItemsSplitPersonalAndCollaborated(t *testing.T) {
	now := time.Now()
	rs := []gh.Repo{
		{NameWithOwner: "sto/mine", Name: "mine", Owned: true, PushedAt: now.Add(-time.Hour)},
		{NameWithOwner: "org/theirs", Name: "theirs", PushedAt: now},
	}
	var got []string
	// Personal leads even in A→Z, where Collaborated would sort first.
	for _, it := range arrange(repoItems(rs, "sto"), "", "", bySection, aToZ) {
		if it.group {
			got = append(got, it.title)
		} else {
			got = append(got, it.cols[0])
		}
	}
	if g := strings.Join(got, " "); g != "Personal mine  Collaborated org/theirs" {
		t.Fatalf("got %q", g)
	}
}
func TestPRStatus(t *testing.T) {
	p := gh.PR{Base: "main", Mergeable: "CONFLICTING", MergeState: "DIRTY", ReviewDecision: "APPROVED",
		CheckCounts: map[string]int{"FAILURE": 1, "SUCCESS": 9, "SKIPPED": 6, "QUEUED": 0}}
	got := prStatus(p)
	for _, want := range []string{"✗ **Conflicts** with `main`", "✗ **Checks**: 1 failed · 9 passed · 6 skipped", "✓ **Approved**"} {
		if !strings.Contains(got, want) {
			t.Errorf("prStatus missing %q:\n%s", want, got)
		}
	}
	if prState(p) != "conflict" {
		t.Errorf("state = %q, want conflict", prState(p))
	}
	// Nothing failed but something still runs: waiting, not passed.
	if got := prStatus(gh.PR{MergeState: "CLEAN", CheckCounts: map[string]int{"SUCCESS": 2, "IN_PROGRESS": 1}}); !strings.Contains(got, "◌ **Checks**: 1 running · 2 passed") {
		t.Errorf("running checks:\n%s", got)
	}
}

func TestChecksMarkdownFailedFirst(t *testing.T) {
	got := checksMarkdown([]gh.Check{{Name: "ok", State: "SUCCESS"}, {Name: "skip", State: "SKIPPED"},
		{Name: "lint", State: "TIMED_OUT", Required: true}, {Name: "slow", State: "IN_PROGRESS"}})
	lint, slow, ok := strings.Index(got, "lint"), strings.Index(got, "slow"), strings.Index(got, "**ok**")
	if !(lint < slow && slow < ok) || strings.Contains(got, "skip**") || !strings.Contains(got, "– 1 skipped") {
		t.Fatalf("order or skipped wrong:\n%s", got)
	}
	if !strings.Contains(got, "**lint** · timed out · required") {
		t.Fatalf("lint line:\n%s", got)
	}
}

func TestDemoteHeadings(t *testing.T) {
	got := demoteHeadings("### Summary\ntext\n```sh\n# comment\n```\n#41 is not a heading")
	want := "\n**Summary**\n\ntext\n```sh\n# comment\n```\n#41 is not a heading"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
