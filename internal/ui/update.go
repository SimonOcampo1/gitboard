package ui

import (
	"errors"
	"os/exec"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/SimonOcampo1/gitboard/internal/gh"
)

const module = "SimonOcampo1/gitboard"

type updateMsg string // sha of a newer commit on GitHub
type updatedMsg struct {
	sha string
	err error
}

// revision is the commit this binary was built from: vcs.revision for a
// build inside a clone, the hash at the end of the pseudo-version for
// go install ...@latest. "" for a build with local changes, or none known.
func revision(info *debug.BuildInfo) string {
	rev := ""
	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision":
			rev = s.Value
		case s.Key == "vcs.modified" && s.Value == "true":
			return ""
		}
	}
	if rev != "" {
		return rev
	}
	v := info.Main.Version
	if i := strings.LastIndex(v, "-"); i >= 0 && len(v)-i-1 == 12 {
		return v[i+1:]
	}
	return ""
}

// checkUpdate reports the newest commit on GitHub when it comes after the
// one running. Any failure is silent: being offline must not nag.
func checkUpdate(c *gh.Client) tea.Cmd {
	return func() tea.Msg {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return nil
		}
		rev := revision(info)
		if rev == "" {
			return nil
		}
		sha, err := c.NewerThan(module, rev)
		if err != nil || sha == "" {
			return nil
		}
		return updateMsg(sha)
	}
}

// install rebuilds gitboard at sha over the running binary. The new code
// only runs after a restart.
func install(sha string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("go", "install", "github.com/"+module+"@"+sha).CombinedOutput()
		if err != nil && len(out) > 0 {
			err = errors.New(strings.TrimSpace(string(out)))
		}
		return updatedMsg{sha, err}
	}
}
