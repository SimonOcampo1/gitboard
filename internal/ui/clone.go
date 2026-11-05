package ui

import (
	"errors"
	"os"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/SimonOcampo1/gitboard/internal/paths"
)

// clonePopup asks for the Clone destination: typed or pasted with Tab
// completion, or chosen in the Folder picker (ctrl+o).
type clonePopup struct {
	repo    item
	input   textinput.Model
	picking bool
	picker  filepicker.Model
	pickerH int
	err     string
}

type cloneStartMsg struct{ repo, dest, target string }
type cloneCancelMsg struct{}

// width and height are the terminal's; the field and the Folder picker
// shrink to fit them.
func newClonePopup(it item, dir string, width, height int) (*clonePopup, tea.Cmd) {
	in := textinput.New()
	in.Prompt = "Destination: "
	in.SetWidth(max(10, min(48, width-24)))
	in.SetValue(strings.TrimRight(dir, "/") + "/")
	in.CursorEnd()
	return &clonePopup{repo: it, input: in, pickerH: max(3, min(12, height-12))}, in.Focus()
}

func (p *clonePopup) openPicker() tea.Cmd {
	start := paths.Expand(p.input.Value())
	if fi, err := os.Stat(start); err != nil || !fi.IsDir() {
		start, _ = os.UserHomeDir()
	}
	fp := filepicker.New()
	fp.CurrentDirectory = start
	fp.DirAllowed, fp.FileAllowed = true, false
	fp.ShowPermissions, fp.ShowSize = false, false
	fp.AutoHeight = false
	fp.SetHeight(p.pickerH)
	// esc belongs to us (close the picker), not to filepicker's "back".
	fp.KeyMap.Back = key.NewBinding(key.WithKeys("h", "backspace", "left"))
	p.picker, p.picking = fp, true
	return fp.Init()
}

func (p *clonePopup) update(msg tea.Msg) tea.Cmd {
	k, isKey := msg.(tea.KeyPressMsg)
	if p.picking {
		if isKey {
			switch k.String() {
			case "space":
				p.input.SetValue(paths.Tilde(p.picker.CurrentDirectory) + "/")
				p.input.CursorEnd()
				p.picking = false
				return nil
			case "esc":
				p.picking = false
				return nil
			}
		}
		var cmd tea.Cmd
		p.picker, cmd = p.picker.Update(msg)
		return cmd
	}
	if isKey {
		switch k.String() {
		case "esc":
			return func() tea.Msg { return cloneCancelMsg{} }
		case "tab":
			p.input.SetValue(paths.CompleteDir(p.input.Value()))
			p.input.CursorEnd()
			return nil
		case "ctrl+o":
			return p.openPicker()
		case "enter":
			dest := strings.TrimSpace(p.input.Value())
			target, err := paths.CloneTarget(dest, p.repo.name)
			if errors.Is(err, paths.ErrExists) {
				p.err = paths.Tilde(target) + " already exists"
				return nil
			}
			return func() tea.Msg { return cloneStartMsg{repo: p.repo.repo, dest: dest, target: target} }
		}
	}
	p.err = ""
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return cmd
}

// view renders the popup at most width cells wide, wrapping the hints.
func (p *clonePopup) view(width int) string {
	box := popupStyle.Width(width)
	title := brand.Render("Clone " + p.repo.repo)
	if p.picking {
		return box.Render(title + "\n" + faint.Render(paths.Tilde(p.picker.CurrentDirectory)) + "\n\n" +
			p.picker.View() + "\n" + faint.Render("j/k move · l open · h up · space choose this folder · esc back"))
	}
	body := title + "\n" + p.input.View() + "\n"
	if p.err != "" {
		body += errStyle.Render(p.err) + "\n"
	}
	return box.Render(body + "\n" + faint.Render("tab complete · ctrl+o browse · enter clone · esc cancel"))
}
