package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/SimonOcampo1/gitboard/internal/config"
	"github.com/SimonOcampo1/gitboard/internal/gh"
	"github.com/charmbracelet/x/ansi"
)

func TestMouse(t *testing.T) {
	var items []item
	for i := range 60 {
		repo := fmt.Sprintf("me/r%d", i%3)
		items = append(items, item{cols: []string{fmt.Sprintf("commit-%02d", i), repo, "abc", "1d"},
			repo: repo, sha: "abcdef0", at: time.Unix(int64(i), 0)})
	}
	m := New(&gh.Client{Login: "me"}, config.Config{}, "")
	m.active = vCommits
	var tm tea.Model = m
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	tm, _ = tm.Update(loadedMsg{view: vCommits, items: items})
	m = tm.(Model)

	// Scrolled away from the top, a click must still land on the row drawn there.
	for range 40 {
		tm, _ = tm.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	}
	m = tm.(Model)
	if m.tables[vCommits].Cursor() < 30 {
		t.Fatalf("wheel moved the cursor to %d only", m.tables[vCommits].Cursor())
	}
	clicked := 0
	for y := 1; y < m.bodyHeight()+1; y++ {
		m = tm.(Model)
		line := ansi.Strip(strings.Split(m.tables[vCommits].View(), "\n")[y-1])
		tm, _ = tm.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: y})
		if !strings.Contains(line, "commit-") {
			continue
		}
		after := tm.(Model)
		it, _ := after.selected()
		if !strings.Contains(line, it.cols[0]) {
			t.Fatalf("clicked line %d %q, selected %s", y, line, it.cols[0])
		}
		clicked++
	}
	if clicked == 0 {
		t.Fatal("no row was clicked")
	}

	_, spans := m.headerTabs()
	tm, _ = tm.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: spans[vIssues][0] + 1, Y: 0})
	if a := tm.(Model).active; a != vIssues {
		t.Fatalf("tab click: active %d", a)
	}
}

func TestDetail(t *testing.T) {
	m := New(&gh.Client{Login: "me"}, config.Config{}, "")
	m.active = vPRs
	var tm tea.Model = m
	// Narrow: no Sidebar, yet enter still shows the detail.
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	tm, _ = tm.Update(loadedMsg{view: vPRs, items: []item{{cols: []string{"fix", "r", "#7", "open", "none", "1d"},
		repo: "me/r", number: 7, url: "u", md: "PR body here", at: time.Now()}}})
	tm, _ = tm.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if v := ansi.Strip(tm.View().Content); !strings.Contains(v, "PR body here") {
		t.Fatalf("detail does not show the body:\n%s", v)
	}
	tm, _ = tm.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if v := ansi.Strip(tm.View().Content); strings.Contains(v, "PR body here") || !strings.Contains(v, "#7") {
		t.Fatalf("esc did not go back to the table:\n%s", v)
	}
}

// Enter, a double click and every relayout (?, b, p, resize) keep the
// row the cursor is on.
func TestDetailOpensTheSelectedRow(t *testing.T) {
	var items []item
	for i := range 30 {
		items = append(items, item{cols: []string{fmt.Sprintf("pr-%02d", i), "r", "#1", "open", "none", "1d"},
			repo: "me/r", number: i, url: fmt.Sprint(i), md: fmt.Sprintf("body of pr-%02d", i), at: time.Unix(int64(100-i), 0)})
	}
	m := New(&gh.Client{Login: "me"}, config.Config{}, "")
	m.active = vPRs
	var tm tea.Model = m
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	tm, _ = tm.Update(loadedMsg{view: vPRs, items: items})
	for range 12 {
		tm, _ = tm.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	m = tm.(Model)
	want, _ := m.selected()
	tm, _ = tm.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if v := ansi.Strip(tm.View().Content); !strings.Contains(v, "body of "+want.cols[0]) {
		t.Fatalf("enter on %s opened:\n%s", want.cols[0], v)
	}
	tm, _ = tm.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm, _ = tm.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = tm.(Model)
	if got, _ := m.selected(); got.url != want.url {
		t.Fatalf("after esc and b the cursor is on %s, want %s", got.cols[0], want.cols[0])
	}

	// Double click on a row opens it.
	m = tm.(Model)
	top, cur := m.cursorLine()
	y := 1 + top + cur - 2 // two rows above the cursor
	tm, _ = tm.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: y})
	m = tm.(Model)
	clicked, _ := m.selected()
	if clicked.url == want.url {
		t.Fatalf("the click did not move off %s", want.cols[0])
	}
	tm, _ = tm.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: y})
	if v := ansi.Strip(tm.View().Content); !tm.(Model).detail || !strings.Contains(v, "body of "+clicked.cols[0]) {
		t.Fatalf("double click on %s did not open it:\n%s", clicked.cols[0], v)
	}
}
