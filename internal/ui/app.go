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
	"github.com/cli/go-gh/v2/pkg/browser"
)

// The Sidebar opens on its own from this width up, until p toggles it.
const sidebarMinWidth = 120

type Model struct {
	client  *gh.Client
	cfg     config.Config
	cfgPath string

	active  int
	all     [5][]item
	shown   [5][]item
	tables  [5]table.Model
	loading [5]bool
	errs    [5]error

	filter    textinput.Model
	filtering bool

	sidebarToggled bool // p was pressed: stop following the width
	sidebarOn      bool
	vp             viewport.Model
	extra          map[string]string // on-demand Sidebar markdown, by item url
	requested      map[string]bool
	rendered       map[string]string // glamour output, by url + width

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
	if i < 0 || i >= len(rows) {
		return item{}, false
	}
	return rows[i], true
}

func (m *Model) setRows(v int) {
	q := ""
	if v == m.active {
		q = m.filter.Value()
	}
	m.shown[v] = filterItems(m.all[v], q)
	rows := make([]table.Row, len(m.shown[v]))
	for i, it := range m.shown[v] {
		rows[i] = it.cols
	}
	m.tables[v].SetRows(rows)
	if m.tables[v].Cursor() >= len(rows) {
		m.tables[v].SetCursor(max(0, len(rows)-1))
	}
}

func (m *Model) layout() {
	if !m.sidebarToggled {
		m.sidebarOn = m.width >= sidebarMinWidth
	}
	bodyH := max(3, m.height-3) // header + status + help
	tw := m.width
	if m.sidebarOn {
		tw = m.width * 55 / 100
		m.vp.SetWidth(m.width - tw - 2)
		m.vp.SetHeight(bodyH - 2)
	}
	for i := range m.tables {
		m.tables[i].SetColumns(fitColumns(views[i].columns, tw))
		m.tables[i].SetWidth(tw)
		m.tables[i].SetHeight(bodyH)
	}
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
	w := m.vp.Width() - 2
	key := fmt.Sprintf("%s|%d|%d", it.url, w, len(md))
	out, ok := m.rendered[key]
	if !ok {
		// The dark style is fixed for now; detecting a light terminal is left for later.
		r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(w))
		if err == nil {
			out, err = r.Render(md)
		}
		if err != nil {
			out = md
		}
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

// localClone is where the selected repo would be if it was cloned into the
// last Clone destination.
// It only looks in the last destination, so a clone made elsewhere is not found.
func (m *Model) localClone(it item) (string, bool) {
	dir := filepath.Join(paths.Expand(m.cfg.CloneDir), it.name)
	fi, err := os.Stat(dir)
	return dir, err == nil && fi.IsDir()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, m.refreshSidebar()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case loadedMsg:
		m.loading[msg.view] = false
		m.errs[msg.view] = msg.err
		m.all[msg.view] = msg.items
		m.setRows(msg.view)
		if msg.view == m.active {
			return m, m.refreshSidebar()
		}
		return m, nil

	case extraMsg:
		if msg.err != nil {
			m.extra[msg.url] = "_Could not load: " + msg.err.Error() + "_"
		} else {
			m.extra[msg.url] = msg.md
		}
		return m, m.refreshSidebar()

	case statusMsg:
		m.status = string(msg)
		return m, nil

	case doneMsg:
		m.status = msg.status
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
		}
		if msg.view >= 0 {
			return m, m.load(msg.view)
		}
		return m, nil

	case cloneCancelMsg:
		m.clone = nil
		return m, nil

	case cloneStartMsg:
		m.clone = nil
		m.cfg.CloneDir = msg.dest
		_ = config.Save(m.cfgPath, m.cfg)
		return m, m.exec(-1, "cloned into "+paths.Tilde(msg.target), exec.Command("gh", "repo", "clone", msg.repo, msg.target))
	}

	if m.clone != nil {
		return m, m.clone.update(msg)
	}

	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.filtering {
		return m, m.updateFilter(k)
	}
	return m, m.handleKey(k)
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
	case "tab":
		return m.switchTo((m.active + 1) % len(views))
	case "shift+tab":
		return m.switchTo((m.active + len(views) - 1) % len(views))
	case "/":
		m.filtering = true
		return m.filter.Focus()
	case "esc":
		m.filter.SetValue("")
		m.setRows(m.active)
		return m.refreshSidebar()
	case "p":
		m.sidebarToggled, m.sidebarOn = true, !m.sidebarOn
		m.layout()
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
	if m.tables[m.active].Cursor() != before {
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
			if err := browser.New("", io.Discard, io.Discard).Browse(url); err != nil {
				return statusMsg("error: " + err.Error())
			}
			return statusMsg("opened " + url)
		}, true
	}
	switch m.active {
	case vRepos:
		switch key {
		case "c":
			p, cmd := newClonePopup(it, m.cfg.CloneDir)
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

func (m Model) header() string {
	var tabs []string
	for i, v := range views {
		label := fmt.Sprintf("%d %s", i+1, v.name)
		if !m.loading[i] && m.errs[i] == nil {
			label += fmt.Sprintf(" %d", len(m.all[i]))
		}
		if i == m.active {
			tabs = append(tabs, tabActive.Render(label))
		} else {
			tabs = append(tabs, tabIdle.Render(label))
		}
	}
	left := brand.Render("gitboard") + strings.Join(tabs, "")
	right := faint.Render("@"+m.client.Login) + " "
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
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

func (m Model) footer() string {
	status := m.status
	if m.filtering || m.filter.Value() != "" {
		status = m.filter.View()
	}
	line := faint.Render("1-5 views · j/k move · / filter · o open · p sidebar · r refresh · q quit · ") + views[m.active].help
	if strings.HasPrefix(status, "error") || strings.HasPrefix(status, "could not") {
		status = errStyle.Render(status)
	}
	return status + "\n" + line
}

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}
	body := m.body()
	bodyH := max(3, m.height-3)
	if m.clone != nil {
		body = lipgloss.Place(m.width, bodyH, lipgloss.Center, lipgloss.Center, m.clone.view())
	} else {
		body = lipgloss.NewStyle().Height(bodyH).MaxHeight(bodyH).Render(body)
	}
	v := tea.NewView(m.header() + "\n" + body + "\n" + m.footer())
	v.AltScreen = true
	return v
}
