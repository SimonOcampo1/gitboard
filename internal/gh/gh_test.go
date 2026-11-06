package gh

import (
	"strings"
	"testing"
	"time"
)

func TestSearchQuery(t *testing.T) {
	cases := map[Kind][]string{
		KindPRs:     {"is:pr", "is:open", "author:sto"},
		KindReviews: {"is:pr", "is:open", "review-requested:sto", "-author:sto"},
		KindIssues:  {"is:issue", "is:open", "assignee:sto"},
	}
	for k, want := range cases {
		q := SearchQuery(k, "sto")
		for _, w := range want {
			if !strings.Contains(q, w) {
				t.Errorf("SearchQuery(%d) = %q, missing %q", k, q, w)
			}
		}
		if !strings.Contains(q, "archived:false") {
			t.Errorf("SearchQuery(%d) = %q, should skip archived repos", k, q)
		}
	}
	// A PR you authored must never land in Reviews by way of the PRs query.
	if strings.Contains(SearchQuery(KindPRs, "sto"), "review-requested") {
		t.Error("PRs query should not filter by review requests")
	}
}

func TestContributionYears(t *testing.T) {
	created := time.Date(2023, 2, 9, 17, 0, 0, 0, time.UTC)
	now := time.Date(2025, 11, 6, 12, 0, 0, 0, time.UTC)
	ys := contributionYears(created, now)
	if len(ys) != 3 || ys[0][0].Year() != 2025 || ys[2][0].Year() != 2023 {
		t.Fatalf("years = %v", ys)
	}
	// The current year stops at now; past years run to Dec 31, under a year each.
	if !ys[0][1].Equal(now) || ys[1][1] != time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC) {
		t.Fatalf("ends = %v, %v", ys[0][1], ys[1][1])
	}
}
