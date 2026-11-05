package ui

import (
	"fmt"
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
)

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
	items := []item{
		{repo: "o/b", cols: []string{"fix login"}},
		{repo: "o/a", cols: []string{"add tests"}},
		{repo: "o/b", cols: []string{"add docs"}},
	}
	if got := arrange(items, "", "o/b", false); len(got) != 2 || got[0].cols[0] != "fix login" {
		t.Fatalf("repo filter = %v", got)
	}
	// Grouped: o/b first (it has the newest row), rows keep their order
	// inside the group, a title row opens each group and a blank row
	// separates them.
	got := arrange(items, "", "", true)
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
	if got := arrange(items, "add", "o/b", false); len(got) != 1 || got[0].cols[0] != "add docs" {
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
