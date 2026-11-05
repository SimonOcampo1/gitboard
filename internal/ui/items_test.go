package ui

import (
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
	cols := fitColumns([]table.Column{{Title: "Name"}, {Title: "Lang", Width: 10}}, 50)
	// 50 - (10+2) - (0+2) = 36
	if cols[0].Width != 36 || cols[1].Width != 10 {
		t.Fatalf("widths = %d, %d", cols[0].Width, cols[1].Width)
	}
	if narrow := fitColumns([]table.Column{{Title: "Name"}, {Title: "Lang", Width: 10}}, 5); narrow[0].Width != 10 {
		t.Fatalf("flexible column should keep a 10-cell floor, got %d", narrow[0].Width)
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
