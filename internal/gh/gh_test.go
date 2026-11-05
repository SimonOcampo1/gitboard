package gh

import (
	"strings"
	"testing"
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
