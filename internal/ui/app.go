// Package ui is the board itself: five Views over one table, a Sidebar,
// and the Quick actions of each View.
package ui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/SimonOcampo1/gitboard/internal/config"
	"github.com/SimonOcampo1/gitboard/internal/gh"
	"github.com/SimonOcampo1/gitboard/internal/paths"
	"github.com/charmbracelet/x/ansi"
	"github.com/cli/browser"
)

const (
	// The Sidebar opens on its own from this width up, until p toggles it.
	sidebarMinWidth = 120
	// Below this, the Sidebar does not fit next to a usable table.
	sidebarMinCols = 24
	tableMinWidth  = 40
)

type Model struct {
	client  *gh.Client
	cfg     config.Config
	cfgPath string

	active  int
	all     [5][]item
	shown   [5][]item
	keep    [5][]int // visible columns of each view, as indexes into item.cols
	tables  [5]table.Model
	loading [5]bool
	errs    [5]error

	filter    textinput.Model
	filtering bool
	repo      string // when set, every view shows this repo only
	byRepo    bool   // group rows by repo
	showHelp  bool   // ? expands the help line into every key
	order     order  // s cycles it

	sidebarToggled bool // p was pressed: stop following the width
	sidebarWanted  bool
	sidebarOn      bool // wanted and it fits
	vp             viewport.Model
	extra          map[string]string // on-demand Sidebar markdown, by item url
	requested      map[string]bool
	rendered       map[string]string // glamour output, by width + markdown

	clone   *clonePopup
	confirm string // url of the issue waiting for a second x
	status  string
	spin    spinner.Model

	width, height int
}

type loadedMsg struct {
	view  int
	items []item
	err   error
}
type extraMsg struct {
	url, md string
	err     error
}
type doneMsg struct {
	view   int // view to reload, or -1
	status string
	err    error
}
type statusMsg string

func New(c *gh.Client, cfg config.Config, cfgPath string) Model {
	m := Model{client: c, cfg: cfg, cfgPath: cfgPath,
		extra: map[string]string{}, requested: map[string]bool{}, rendered: map[string]string{}}
	for i := range m.tables {
		m.tables[i] = table.New(table.WithFocused(true), table.WithStyles(tableStyles()))
	}
	m.filter = textinput.New()
	m.filter.Prompt = "/"
	m.spin = spinner.New(spinner.WithSpinner(spinner.Dot))
	m.vp = viewport.New()
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spin.Tick}
	for i := range views {
		cmds = append(cmds, m.load(i))
	}
	return tea.Batch(cmds...)
}

func (m *Model) load(v int) tea.Cmd {
	m.loading[v] = true
	c := m.client
	return func() tea.Msg {
		items, err := views[v].load(c)
		return loadedMsg{v, items, err}
	}
}

func (m *Model) selected() (item, bool) {
	rows := m.shown[m.active]
	i := m.tables[m.active].Cursor()
	if i < 0 || i >= len(rows) || rows[i].group {
		return item{}, false
	}
	return rows[i], true
}

// skipGroupRows moves the cursor of view v off a group row, going dir
// (+1 down, -1 up) first and the other way when that runs out.
func (m *Model) skipGroupRows(v, dir int) {
	rows, t := m.shown[v], &m.tables[v]
	for _, d := range []int{dir, -dir} {
		for c := t.Cursor(); c >= 0 && c < len(rows); c += d {
			if !rows[c].group {
				t.SetCursor(c)
				return
			}
		}
	}
}

func (m *Model) setRows(v int) {
	q := ""
	if v == m.active {
		q = m.filter.Value()
	}
	m.shown[v] = arrange(m.all[v], q, m.repo, m.grouped(v), m.order)
	cols := m.tables[v].Columns()
	rows := make([]table.Row, len(m.shown[v]))
	for i, it := range m.shown[v] {
		if it.group {
			rows[i] = groupRow(it, cols, m.client.Login)
			continue
		}
		row := make(table.Row, 0, len(m.keep[v]))
		for j, k := range m.keep[v] {
			// The table has no cell padding (see tableStyles): each cell
			// carries its own leading space and leaves one free at the end.
			row = append(row, " "+ansi.Truncate(it.cols[k], cols[j].Width-2, "…"))
		}
		rows[i] = row
	}
	m.tables[v].SetRows(rows)
	// Clearing the rows (see layout) leaves the cursor at -1: put it back on a row.
	if c := m.tables[v].Cursor(); c < 0 || c >= len(rows) {
		m.tables[v].SetCursor(max(0, min(c, len(rows)-1)))
	}
	m.skipGroupRows(v, 1)
}

// groupRow draws a group row. A spacer is empty; a title row is one line
// across the whole table, the repo and its row count followed by a
// divider, cut into the cells. That only works because cells have no
// padding of their own.
func groupRow(it item, cols []table.Column, login string) table.Row {
	row := make(table.Row, len(cols))
	if it.title == "" {
		return row
	}
	total := 0
	for _, c := range cols {
		total += c.Width
	}
	title := ansi.Truncate(shortRepo(it.title, login)+" · "+fmt.Sprint(it.count), max(1, total-3), "…")
	line := " " + groupStyle.Render(title) + " " + faint.Render(strings.Repeat("─", max(0, total-lipgloss.Width(title)-3)))
	start := 0
	for j, c := range cols {
		row[j] = ansi.Cut(line, start, start+c.Width)
		start += c.Width
	}
	return row
}

// grouped reports whether view v groups its rows by repo. Never in Repos:
// every row there is already a repo.
func (m *Model) grouped(v int) bool {
	return m.byRepo && v != vRepos
}

func (m *Model) setAllRows() {
	for v := range views {
		m.setRows(v)
	}
}

func (m *Model) bodyHeight() int {
	return max(3, m.height-2-len(m.helpLines())) // header + status + help
}

func (m *Model) layout() {
	if !m.sidebarToggled {
		m.sidebarWanted = m.width >= sidebarMinWidth
	}
	bodyH := m.bodyHeight()
	tw := m.width
	m.sidebarOn = false
	if m.sidebarWanted {
		tw = max(tableMinWidth, m.width*55/100)
		if sw := m.width - tw - 2; sw >= sidebarMinCols {
			m.sidebarOn = true
			m.vp.SetWidth(sw)
			m.vp.SetHeight(bodyH - 2)
		} else {
			tw = m.width
		}
	}
	for i := range m.tables {
		// Grouped by repo, the Repo column is what makes the groups visible.
		mustKeep := ""
		if m.grouped(i) {
			mustKeep = "Repo"
		}
		cols, keep := fitColumnsKeeping(views[i].columns, views[i].drop, tw, mustKeep)
		// Cells pad themselves (see setRows), so each column gets its 2
		// cells of padding back as width.
		for j := range cols {
			cols[j].Width += 2
			cols[j].Title = " " + cols[j].Title
		}
		// The table reads rows with the old column count while columns
		// change, so clear them first; setAllRows refills them.
		m.tables[i].SetRows(nil)
		m.keep[i] = keep
		m.tables[i].SetColumns(cols)
		m.tables[i].SetWidth(tw)
		m.tables[i].SetHeight(bodyH)
	}
	m.setAllRows()
}

// refreshSidebar renders the selected item and asks for its on-demand part
// (README, commit files) the first time it is shown.
func (m *Model) refreshSidebar() tea.Cmd {
	it, ok := m.selected()
	if !m.sidebarOn || !ok {
		m.vp.SetContent("")
		return nil
	}
	var cmd tea.Cmd
	if it.fetch != nil && !m.requested[it.url] {
		m.requested[it.url] = true
		c, fetch, url := m.client, it.fetch, it.url
		cmd = func() tea.Msg { md, err := fetch(c); return extraMsg{url, md, err} }
	}
	md := it.md
	if x, ok := m.extra[it.url]; ok {
		md = strings.TrimSpace(md + "\n\n" + x)
	} else if it.fetch != nil {
		md = strings.TrimSpace(md + "\n\n_Loading…_")
	}
	w := m.vp.Width()
	// The key holds the markdown itself: "_Loading…_" and "_No README._" have
	// the same length, so keying by length served the loading text forever.
	key := fmt.Sprintf("%d|%s", w, md)
	out, ok := m.rendered[key]
	if !ok {
		// The dark style is fixed for now; detecting a light terminal is left for later.
		// Glamour adds a 2-cell margin on each side, hence w-4.
		r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(max(10, w-4)))
		if err == nil {
			out, err = r.Render(md)
		}
		if err != nil {
			out = md
		}
		// Long words, URLs and code lines still overflow the wrap: cut them.
		// Glamour can also emit tabs, which the terminal widens past the border.
		lines := strings.Split(strings.ReplaceAll(out, "\t", "    "), "\n")
		for i, l := range lines {
			lines[i] = ansi.Truncate(l, w, "…")
		}
		out = strings.Join(lines, "\n")
		m.rendered[key] = out
	}
	m.vp.SetContent(out)
	m.vp.GotoTop()
	return cmd
}

func (m *Model) switchTo(v int) tea.Cmd {
	m.filter.SetValue("")
	m.setRows(m.active)
	m.active = v
	m.confirm = ""
	return m.refreshSidebar()
}

func (m *Model) exec(view int, status string, c *exec.Cmd) tea.Cmd {
	return tea.ExecProcess(c, func(err error) tea.Msg { return doneMsg{view, status, err} })
}

// openURL uses $BROWSER when it names a program that exists, and the
// system's default browser otherwise (xdg-open, open, or the Windows
// handler). A stale $BROWSER must not stop links from opening.
func openURL(url string) error {
	if b := strings.Fields(os.Getenv("BROWSER")); len(b) > 0 {
		if path, err := exec.LookPath(b[0]); err == nil {
			return exec.Command(path, append(b[1:], url)...).Start()
		}
	}
	browser.Stdout, browser.Stderr = io.Discard, io.Discard
	return browser.OpenURL(url)
}

// localClone is where the selected repo would be if it was cloned into the
// last Clone destination.
// It only looks in the last destination, so a clone made elsewhere is not found.
func (m *Model) localClone(it item) (string, bool) {
	dir := filepath.Join(paths.Expand(m.cfg.CloneDir), it.name)
	fi, err := os.Stat(dir)
	return dir, err == nil && fi.IsDir()
}

// Update never writes `return m, m.someMutatingMethod()`: Go does not fix
// whether m is copied before or after the call runs, so the change could
// be lost. The command is taken first and m returned after.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		cmd = m.refreshSidebar()
		return m, cmd

	case spinner.TickMsg:
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case loadedMsg:
		m.loading[msg.view] = false
		m.errs[msg.view] = msg.err
		m.all[msg.view] = msg.items
		m.setRows(msg.view)
		if msg.view == m.active {
			cmd = m.refreshSidebar()
		}
		return m, cmd

	case extraMsg:
		if msg.err != nil {
			m.extra[msg.url] = "_Could not load: " + msg.err.Error() + "_"
		} else {
			m.extra[msg.url] = msg.md
		}
		cmd = m.refreshSidebar()
		return m, cmd

	case statusMsg:
		m.status = string(msg)
		return m, nil

	case doneMsg:
		m.status = msg.status
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
		}
		if msg.view >= 0 {
			cmd = m.load(msg.view)
		}
		return m, cmd

	case cloneCancelMsg:
		m.clone = nil
		return m, nil

	case cloneStartMsg:
		m.clone = nil
		m.cfg.CloneDir = msg.dest
		_ = config.Save(m.cfgPath, m.cfg)
		cmd = m.exec(-1, "cloned into "+paths.Tilde(msg.target), exec.Command("gh", "repo", "clone", msg.repo, msg.target))
		return m, cmd
	}

	if m.clone != nil {
		cmd = m.clone.update(msg)
		return m, cmd
	}

	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.filtering {
		cmd = m.updateFilter(k)
	} else {
		cmd = m.handleKey(k)
	}
	return m, cmd
}

func (m *Model) updateFilter(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "enter":
		m.filtering = false
		m.filter.Blur()
		return nil
	case "esc":
		m.filtering = false
		m.filter.Blur()
		m.filter.SetValue("")
	default:
		m.filter, _ = m.filter.Update(k)
	}
	m.setRows(m.active)
	m.tables[m.active].SetCursor(0)
	m.skipGroupRows(m.active, 1)
	return m.refreshSidebar()
}

func (m *Model) handleKey(k tea.KeyPressMsg) tea.Cmd {
	it, has := m.selected()
	key := k.String()
	if key != "x" {
		m.confirm = ""
	}
	switch key {
	case "q", "ctrl+c":
		return tea.Quit
	case "1", "2", "3", "4", "5":
		return m.switchTo(int(key[0] - '1'))
	case "tab", "right", "L":
		return m.switchTo((m.active + 1) % len(views))
	case "shift+tab", "left", "H":
		return m.switchTo((m.active + len(views) - 1) % len(views))
	case "/":
		m.filtering = true
		return m.filter.Focus()
	case "esc":
		m.filter.SetValue("")
		m.repo = ""
		m.setAllRows()
		return m.refreshSidebar()
	case "f":
		if m.repo != "" {
			m.repo = ""
		} else if has {
			m.repo = it.repo
		}
		m.setAllRows()
		m.tables[m.active].SetCursor(0)
		m.skipGroupRows(m.active, 1)
		return m.refreshSidebar()
	case "?":
		m.showHelp = !m.showHelp
		m.layout()
		return m.refreshSidebar()
	case "s":
		m.order = (m.order + 1) % orderCount
		m.setAllRows()
		m.tables[m.active].SetCursor(0)
		m.skipGroupRows(m.active, 1)
		return m.refreshSidebar()
	case "b":
		if m.active == vRepos {
			return nil
		}
		m.byRepo = !m.byRepo
		m.layout()
		return m.refreshSidebar()
	case "p":
		m.sidebarToggled, m.sidebarWanted = true, !m.sidebarWanted
		m.layout()
		if m.sidebarWanted && !m.sidebarOn {
			m.status = "the sidebar needs a wider terminal"
		}
		return m.refreshSidebar()
	case "r":
		m.status = ""
		return m.load(m.active)
	case "ctrl+d":
		m.vp.HalfPageDown()
		return nil
	case "ctrl+u":
		m.vp.HalfPageUp()
		return nil
	}

	if has {
		if cmd, handled := m.quickAction(key, it); handled {
			return cmd
		}
	}

	before := m.tables[m.active].Cursor()
	var cmd tea.Cmd
	m.tables[m.active], cmd = m.tables[m.active].Update(k)
	if after := m.tables[m.active].Cursor(); after != before {
		dir := 1
		if after < before {
			dir = -1
		}
		m.skipGroupRows(m.active, dir)
		return tea.Batch(cmd, m.refreshSidebar())
	}
	return cmd
}

// quickAction runs the single-key command of the active View on it.
func (m *Model) quickAction(key string, it item) (tea.Cmd, bool) {
	num := fmt.Sprint(it.number)
	if key == "o" {
		url := it.url
		return func() tea.Msg {
			if err := openURL(url); err != nil {
				return statusMsg("error: " + err.Error())
			}
			return statusMsg("opened " + url)
		}, true
	}
	switch m.active {
	case vRepos:
		switch key {
		case "c":
			p, cmd := newClonePopup(it, m.cfg.CloneDir, m.width, m.height)
			m.clone = p
			return cmd, true
		case "g", "n":
			dir, ok := m.localClone(it)
			if !ok {
				m.status = it.name + " is not cloned in " + m.cfg.CloneDir
				return nil, true
			}
			c := exec.Command("lazygit", "-p", dir)
			if key == "n" {
				c = exec.Command("gh", "pr", "create")
				c.Dir = dir
			}
			return m.exec(vPRs, "", c), true
		}
	case vPRs, vReviews:
		switch key {
		case "d":
			return m.exec(-1, "", exec.Command("gh", "pr", "diff", num, "-R", it.repo)), true
		case "m":
			if m.active == vPRs {
				return m.exec(vPRs, "", exec.Command("gh", "pr", "merge", num, "-R", it.repo)), true
			}
		case "a":
			if m.active == vReviews {
				return m.exec(vReviews, "approved "+it.repo+"#"+num, exec.Command("gh", "pr", "review", num, "-R", it.repo, "--approve")), true
			}
		}
	case vCommits:
		if key == "y" {
			m.status = "copied " + it.sha[:7]
			return tea.SetClipboard(it.sha), true
		}
	case vIssues:
		if key == "x" {
			if m.confirm != it.url {
				m.confirm = it.url
				m.status = "press x again to close " + it.repo + "#" + num
				return nil, true
			}
			m.confirm = ""
			c := exec.Command("gh", "issue", "close", num, "-R", it.repo)
			return func() tea.Msg {
				if out, err := c.CombinedOutput(); err != nil {
					return doneMsg{vIssues, "", fmt.Errorf("%s", strings.TrimSpace(string(out)))}
				}
				return doneMsg{vIssues, "closed " + it.repo + "#" + num, nil}
			}, true
		}
	}
	return nil, false
}

// header drops detail until it fits: first the login, then the counts,
// then the inactive tabs. Whatever is still too wide is cut.
func (m Model) header() string {
	tabs := func(counts, onlyActive bool) string {
		var out []string
		for i, v := range views {
			if onlyActive && i != m.active {
				continue
			}
			label := v.name
			if counts && !m.loading[i] && m.errs[i] == nil {
				label += " " + fmt.Sprint(rowCount(m.shown[i]))
			}
			if i == m.active {
				out = append(out, tabActive.Render(label))
			} else {
				out = append(out, tabIdle.Render(label))
			}
		}
		return strings.Join(out, "")
	}
	login := faint.Render("@"+m.client.Login) + " "
	for _, try := range []struct{ counts, onlyActive, login bool }{
		{true, false, true}, {true, false, false}, {false, false, false}, {true, true, false}, {false, true, false},
	} {
		left := brand.Render("gitboard") + tabs(try.counts, try.onlyActive)
		if !try.login {
			if lipgloss.Width(left) <= m.width {
				return left
			}
			continue
		}
		if gap := m.width - lipgloss.Width(left) - lipgloss.Width(login); gap >= 1 {
			return left + strings.Repeat(" ", gap) + login
		}
	}
	return ansi.Truncate(brand.Render("gitboard")+tabs(false, true), m.width, "…")
}

func (m Model) body() string {
	v := m.active
	switch {
	case m.loading[v] && len(m.all[v]) == 0:
		return m.spin.View() + " loading " + views[v].name + "…"
	case m.errs[v] != nil:
		return errStyle.Render("could not load " + views[v].name + ": " + m.errs[v].Error())
	case len(m.shown[v]) == 0:
		return faint.Render("  nothing here")
	}
	t := m.tables[v].View()
	if !m.sidebarOn {
		return t
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, t, panel.Render(m.vp.View()))
}

// helpLines lists the keys of the active view first, then the general
// ones, wrapped to the terminal width. ? swaps the short list for all of them.
func (m *Model) helpLines() []string {
	hints := append([]hint{}, views[m.active].help...)
	// In Repos, f focuses the other views on the selected repo, and b
	// (grouping) does not apply: every row is already a repo.
	repoKeys := []hint{{"f", "repo"}, {"b", "group"}}
	if m.showHelp {
		repoKeys = []hint{{"f", "only this repo"}, {"b", "group by repo"}}
	}
	repoKeys = append(repoKeys, hint{"s", "sort"})
	if m.active == vRepos {
		repoKeys = []hint{{"f", "focus"}, {"s", "sort"}}
		if m.showHelp {
			repoKeys = []hint{{"f", "focus the other views on this repo"}, {"s", "sort"}}
		}
	}
	if m.showHelp {
		repoKeys[len(repoKeys)-1].label = "sort: newest, oldest, A→Z, Z→A"
	}
	if m.showHelp {
		hints = append(hints, hint{"tab ←→", "view"}, hint{"j k", "move"}, hint{"/", "search"})
		hints = append(hints, repoKeys...)
		hints = append(hints, hint{"esc", "clear search and repo"}, hint{"o", "open"}, hint{"p", "sidebar"},
			hint{"^d ^u", "scroll sidebar"}, hint{"r", "reload"}, hint{"?", "less"}, hint{"q", "quit"})
	} else {
		hints = append(hints, hint{"/", "search"})
		hints = append(hints, repoKeys...)
		hints = append(hints, hint{"o", "open"}, hint{"?", "more"}, hint{"q", "quit"})
	}
	width := max(20, m.width)
	var lines []string
	line := ""
	for _, h := range hints {
		cell := keyStyle.Render(h.key) + " " + faint.Render(h.label)
		switch {
		case line == "":
			line = cell
		case lipgloss.Width(line)+2+lipgloss.Width(cell) <= width:
			line += "  " + cell
		default:
			lines = append(lines, line)
			line = cell
		}
	}
	return append(lines, line)
}

func (m Model) footer() string {
	var parts []string
	if m.filtering || m.filter.Value() != "" {
		parts = append(parts, m.filter.View())
	}
	if m.repo != "" {
		parts = append(parts, brand.Render("repo: "+m.repo)+faint.Render(" (f to clear)"))
	}
	if m.order != newestFirst {
		parts = append(parts, faint.Render("sorted "+m.order.String()))
	}
	if m.grouped(m.active) {
		parts = append(parts, faint.Render("grouped by repo"))
	}
	if m.status != "" {
		s := m.status
		if strings.HasPrefix(s, "error") || strings.HasPrefix(s, "could not") {
			s = errStyle.Render(s)
		}
		parts = append(parts, s)
	}
	status := ansi.Truncate(strings.Join(parts, "  "), m.width, "…")
	return status + "\n" + strings.Join(m.helpLines(), "\n")
}

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}
	bodyH := m.bodyHeight()
	var body string
	if m.clone != nil {
		popup := m.clone.view(max(20, min(70, m.width-2)))
		body = lipgloss.Place(m.width, bodyH, lipgloss.Center, lipgloss.Center, popup)
	} else {
		body = lipgloss.NewStyle().Height(bodyH).MaxHeight(bodyH).Render(m.body())
	}
	v := tea.NewView(m.header() + "\n" + body + "\n" + m.footer())
	v.AltScreen = true
	return v
}
