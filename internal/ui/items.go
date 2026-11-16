package ui

import (
	"fmt"
	"sort"
	"strconv"
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
	case p.Mergeable == "CONFLICTING":
		return "conflict"
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
		md := fmt.Sprintf("# %s\n\n`%s#%d` · `%s` → `%s`\n\n%s\n\n---\n\n%s", p.Title, p.Repo, p.Number, p.Head, p.Base, prStatus(p), p.Body)
		repo, number := p.Repo, p.Number
		out = append(out, item{
			cols: []string{p.Title, shortRepo(p.Repo, login), fmt.Sprintf("#%d", p.Number), prState(p), strings.ToLower(checks), ago(p.UpdatedAt)}, at: p.UpdatedAt,
			url: p.URL, repo: p.Repo, number: p.Number, md: md,
			fetch: func(c *gh.Client) (string, error) {
				checks, cs, err := c.PRDetail(repo, number)
				return checksMarkdown(checks) + "\n\n" + conversationMarkdown(cs), err
			},
		})
	}
	return out
}

// Check buckets: what a reader wants to know of a check, worst first.
const (
	failed = iota
	running
	passed
	skipped
)

var (
	bucketIcon = [...]string{"✗", "◌", "✓", "–"}
	bucketWord = [...]string{"failed", "running", "passed", "skipped"}
)

func checkBucket(state string) int {
	switch state {
	case "SUCCESS", "COMPLETED":
		return passed
	case "SKIPPED", "NEUTRAL", "STALE":
		return skipped
	case "FAILURE", "ERROR", "TIMED_OUT", "STARTUP_FAILURE", "CANCELLED", "ACTION_REQUIRED":
		return failed
	}
	return running // QUEUED, IN_PROGRESS, PENDING, WAITING, EXPECTED...
}

// prStatus answers "can this merge, and if not why" in three lines:
// merge state, checks and review, each led by ✓ fine, ✗ in the way,
// ! a warning, ◌ waiting or – nothing to say.
func prStatus(p gh.PR) string {
	base := "`" + p.Base + "`"
	var merge string
	switch {
	case p.MergeState == "DIRTY" || p.Mergeable == "CONFLICTING":
		merge = "✗ **Conflicts** with " + base + ": resolve them before merging"
	case p.Draft:
		merge = "◌ **Draft**: mark it ready for review to merge"
	case p.MergeState == "BEHIND":
		merge = "◌ **Behind** " + base + ": update the branch before merging"
	case p.MergeState == "BLOCKED":
		merge = "✗ **Blocked**: a required review or check is missing"
	case p.MergeState == "UNSTABLE":
		merge = "! **Can merge**, but some checks are not passing"
	case p.MergeState == "CLEAN" || p.MergeState == "HAS_HOOKS":
		merge = "✓ **Ready to merge** into " + base
	default:
		merge = "◌ **Merge state unknown**: GitHub is still working it out (`r` reloads)"
	}

	var n [4]int
	for state, c := range p.CheckCounts {
		n[checkBucket(state)] += c
	}
	var parts []string
	for b, c := range n {
		if c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, bucketWord[b]))
		}
	}
	checks := "– **No checks**"
	if len(parts) > 0 {
		icon := bucketIcon[passed]
		for _, b := range []int{failed, running} {
			if n[b] > 0 {
				icon = bucketIcon[b]
				break
			}
		}
		checks = icon + " **Checks**: " + strings.Join(parts, " · ")
	}

	review := map[string]string{"APPROVED": "✓ **Approved**", "CHANGES_REQUESTED": "✗ **Changes requested**",
		"REVIEW_REQUIRED": "◌ **Review required**"}[p.ReviewDecision]
	if review == "" {
		review = "– **No review required**"
	}
	return "- " + merge + "\n- " + checks + "\n- " + review
}

// checksMarkdown lists every check, failed first. Skipped ones only count:
// their names are noise.
func checksMarkdown(cs []gh.Check) string {
	if len(cs) == 0 {
		return "---\n\n_No checks._"
	}
	sort.SliceStable(cs, func(i, j int) bool { return checkBucket(cs[i].State) < checkBucket(cs[j].State) })
	var b strings.Builder
	fmt.Fprintf(&b, "---\n\n## Checks · %d\n", len(cs))
	nSkipped := 0
	for _, c := range cs {
		bk := checkBucket(c.State)
		if bk == skipped {
			nSkipped++
			continue
		}
		fmt.Fprintf(&b, "\n- %s **%s**", bucketIcon[bk], c.Name)
		// The icon already says success or failure; other states add something.
		if c.State != "SUCCESS" && c.State != "FAILURE" {
			b.WriteString(" · " + strings.ToLower(strings.ReplaceAll(c.State, "_", " ")))
		}
		if c.Required && bk != passed {
			b.WriteString(" · required")
		}
	}
	if nSkipped > 0 {
		fmt.Fprintf(&b, "\n- %s %d skipped", bucketIcon[skipped], nSkipped)
	}
	return b.String()
}

// conversationMarkdown renders a PR's comments and reviews for the Sidebar.
func conversationMarkdown(cs []gh.Comment) string {
	if len(cs) == 0 {
		return "---\n\n_No comments or reviews._"
	}
	verb := map[string]string{"": "commented", "APPROVED": "approved", "CHANGES_REQUESTED": "requested changes",
		"COMMENTED": "reviewed", "DISMISSED": "review dismissed", "PENDING": "pending review"}
	var b strings.Builder
	fmt.Fprintf(&b, "---\n\n## Conversation · %d", len(cs))
	for _, c := range cs {
		v, ok := verb[c.State]
		if !ok {
			v = strings.ToLower(c.State)
		}
		// Each comment is its own heading, not a quote or a list item:
		// glamour wraps nested blocks past the width and breaks their lines.
		fmt.Fprintf(&b, "\n\n### @%s %s · %s", c.Author, v, ago(c.At))
		if body := strings.TrimSpace(c.Body); body != "" {
			b.WriteString("\n\n" + demoteHeadings(body))
		}
		for _, in := range c.Inline {
			loc := in.Path
			if in.Line > 0 {
				loc += fmt.Sprintf(":%d", in.Line)
			}
			b.WriteString("\n\n`" + loc + "`\n\n" + demoteHeadings(strings.TrimSpace(in.Body)))
		}
	}
	return b.String()
}

// demoteHeadings turns the headings of a comment into bold lines, so they
// read as part of the comment and not as the next one. Code blocks keep theirs.
func demoteHeadings(md string) string {
	lines := strings.Split(md, "\n")
	code := false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			code = !code
		}
		if h := strings.TrimLeft(t, "#"); !code && len(h) < len(t) && len(t)-len(h) <= 6 && strings.HasPrefix(h, " ") {
			lines[i] = "\n**" + strings.TrimSpace(h) + "**\n"
		}
	}
	return strings.Join(lines, "\n")
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
			cols: []string{i.Title, shortRepo(i.Repo, login), fmt.Sprintf("#%d", i.Number), labels, ago(i.UpdatedAt)}, at: i.UpdatedAt,
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
// A q like "#42" asks for PR or issue 42 exactly, not 420 or 142.
func filterItems(items []item, q string) []item {
	if q == "" {
		return items
	}
	q = strings.ToLower(q)
	n, err := strconv.Atoi(strings.TrimPrefix(q, "#"))
	exact := strings.HasPrefix(q, "#") && err == nil
	var out []item
	for _, it := range items {
		if exact && it.number == n || !exact && strings.Contains(strings.ToLower(strings.Join(it.cols, " ")), q) {
			out = append(out, it)
		}
	}
	return out
}
