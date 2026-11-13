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
