<p align="center">
  <img src="docs/banner.png" alt="gitboard" width="100%">
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="Bubble Tea" src="https://img.shields.io/badge/Bubble_Tea-v2-FF5F87?style=flat-square">
  <img alt="GitHub CLI" src="https://img.shields.io/badge/gh_CLI-181717?style=flat-square&logo=github&logoColor=white">
</p>

<p align="center">
  <a href="#features">Features</a> &nbsp;•&nbsp;
  <a href="#keys">Keys</a> &nbsp;•&nbsp;
  <a href="#stack">Stack</a> &nbsp;•&nbsp;
  <a href="#structure">Structure</a> &nbsp;•&nbsp;
  <a href="#install">Install</a>
</p>

gitboard puts your GitHub account in one terminal window: your repos, your open PRs, your latest commits, the reviews waiting on you and the issues assigned to you, each one a key away. It sits next to `gh-dash` (PRs and issues) and `lazygit` (local git) and covers the part neither does: the account as a whole.

<p align="center">
  <img src="docs/screenshot.png" alt="gitboard showing the Repos view with a README in the sidebar" width="100%">
</p>

## Features

- **Five views, one table.** Repos, PRs, Commits, Reviews and Issues, on keys `1`-`5`, all loaded in parallel on start. Every view is flattened into the same row type, so the filter (`/`) and the sidebar work the same everywhere.
- **No login of its own.** It reuses the session of the `gh` CLI through `go-gh`. If `gh auth status` is fine, gitboard is too.
- **Clone without leaving the board.** `c` opens a prompt that starts at the last folder you cloned into, completes paths with `Tab` like a shell, and opens a folders-only explorer with `Ctrl+O`. It refuses to clone into a folder that already exists.
- **A sidebar that loads on demand.** The README of a repo, the body and checks of a PR, the full message and changed files of a commit. Fetched the first time a row is selected, rendered with Glamour, and cached.
- **Hands off to the tools that already do it well.** Diffs, merges and approvals run through `gh`; `g` opens `lazygit` and `n` runs `gh pr create` inside your local clone.
- **Every commit, automated ones included.** Commits come from GitHub's commit search, which indexes each repo's default branch: a commit on a feature branch shows up once it is merged.

## Keys

| Where | Key | Action |
|---|---|---|
| Everywhere | `1`-`5` / `Tab` | Switch view |
| | `j` `k` | Move |
| | `/` | Filter rows (`Enter` keeps it, `Esc` clears it) |
| | `o` | Open in the browser |
| | `p` | Toggle the sidebar (open by default from 120 columns) |
| | `Ctrl+D` `Ctrl+U` | Scroll the sidebar |
| | `r` | Reload the current view |
| | `q` | Quit |
| Repos | `c` | Clone |
| | `g` | `lazygit` in the local clone |
| | `n` | `gh pr create` in the local clone |
| PRs | `d` / `m` | Diff / merge |
| Commits | `y` | Copy the SHA (OSC 52) |
| Reviews | `d` / `a` | Diff / approve |
| Issues | `x` `x` | Close (press twice) |

## Stack

| Layer | Technology | Why |
|---|---|---|
| UI | Bubble Tea v2, Bubbles, Lip Gloss | The same stack as `gh-dash`. ANSI colors only, so the board takes your terminal's palette. |
| Markdown | Glamour | Renders READMEs and PR bodies in the sidebar. |
| GitHub | `go-gh` | Official client library of the `gh` CLI: its auth, GraphQL for repos, PRs and issues, REST for commit search. |
| Config | `yaml.v3` | One value: the last clone destination, in `~/.config/gitboard/config.yml`. |

## Structure

```
main.go                 checks the gh session, starts the program
internal/gh/            GitHub queries; one search string per view
internal/ui/
  app.go                the board: views, sidebar, filter, quick actions
  items.go              every view flattened into one row type
  views.go              columns, help line and loader of each view
  clone.go              clone prompt and folder picker
internal/paths/         ~ expansion, Tab completion, never-overwrite check
internal/config/        the remembered clone destination
CONTEXT.md              glossary: view, quick action, clone destination...
```

## Install

```bash
gh auth login                                  # once, if gh is not set up yet
go install github.com/SimonOcampo1/gitboard@latest
gitboard
```

You need Go 1.27+ and an authenticated `gh`. `g` needs `lazygit` on your `PATH`. `g` and `n` look for the clone at `<last clone destination>/<repo name>`; a repo cloned somewhere else is not found yet.
