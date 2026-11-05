// gitboard is a terminal dashboard for your whole GitHub account.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/SimonOcampo1/gitboard/internal/config"
	"github.com/SimonOcampo1/gitboard/internal/gh"
	"github.com/SimonOcampo1/gitboard/internal/ui"
	"github.com/cli/go-gh/v2/pkg/auth"
)

func main() {
	if token, _ := auth.TokenForHost("github.com"); token == "" {
		fmt.Fprintln(os.Stderr, "gitboard: not logged in to GitHub. Run: gh auth login")
		os.Exit(1)
	}
	client, err := gh.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitboard:", err)
		os.Exit(1)
	}
	path := config.Path()
	if _, err := tea.NewProgram(ui.New(client, config.Load(path), path)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "gitboard:", err)
		os.Exit(1)
	}
}
