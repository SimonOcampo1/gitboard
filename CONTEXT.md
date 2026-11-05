# gitboard

A terminal dashboard for your whole GitHub account: repos, pull requests, commits, reviews and issues in one TUI, with quick actions on each.

## Language

**View**:
One of the five tabs of the board, switched with keys `1`-`5`.
_Avoid_: section, page, screen

**Repos** (view 1):
The repositories owned by the signed-in account.

**PRs** (view 2):
Open pull requests authored by the signed-in account, across all repos.
_Avoid_: my pulls

**Commits** (view 3):
The latest commits authored by the signed-in account, across all repos, on each repo's default branch only. Every commit is shown, automated ones included.

**Reviews** (view 4):
Open pull requests where the signed-in account is a requested reviewer.
_Avoid_: review requests (as a view name)

**Issues** (view 5):
Open issues assigned to the signed-in account.

**Quick action**:
A single-key command applied to the selected row of a view (clone, open, merge, copy SHA...).
_Avoid_: shortcut, hotkey

**Clone destination**:
The parent folder a repo is cloned into; the clone lands at `<destination>/<repo name>`. Defaults to the last one used (`~/repos` the first time).
_Avoid_: clone path, target dir

**Folder picker**:
The directories-only explorer opened from the clone prompt to choose a **Clone destination**.
_Avoid_: file browser

**Sidebar**:
The right-hand panel showing the detail of the selected row (README, PR body, commit message and files, issue body). Toggled with `p`; open by default at 120+ columns.
_Avoid_: preview pane, detail view

## Relationships

- Views 1-3 show **your own work**; views 4-5 show **what others ask of you**
- A **PR** shows up in **Reviews** only when someone else authored it
- A clone never overwrites: if `<destination>/<repo name>` already exists, the clone is refused

## Example dialogue

> **Dev:** "If I open a PR and request myself as a reviewer, does it show in **Reviews**?"
> **Domain expert:** "No: it's yours, so it lives in **PRs**. **Reviews** is only for PRs someone else wrote."

## Flagged ambiguities

- Creating commits and PRs from the TUI is out of the MVP: that is local git work. A **Quick action** may hand off to `gh pr create` or lazygit in the selected repo instead.
