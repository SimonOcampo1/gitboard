// Package paths resolves the folders typed in the clone prompt.
package paths

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrExists = errors.New("already exists")

// Sep is the separator shown and appended in the prompt: \ on Windows, / elsewhere.
const Sep = string(filepath.Separator)

// seps are the separators a typed path may use: / always, \ too on Windows.
const seps = "/" + Sep

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

// Expand turns a leading ~ into the home directory.
func Expand(p string) string {
	if p == "~" {
		return home()
	}
	if len(p) > 1 && p[0] == '~' && strings.ContainsRune(seps, rune(p[1])) {
		return filepath.Join(home(), p[2:])
	}
	return p
}

// Tilde is the inverse of Expand, for showing paths compactly.
func Tilde(p string) string {
	h := home()
	if p == h {
		return "~"
	}
	if strings.HasPrefix(p, h+string(filepath.Separator)) {
		return "~" + p[len(h):]
	}
	return p
}

// CloneTarget is where a clone of repoName into dest lands. It refuses a
// target that already exists, so a clone never overwrites anything.
func CloneTarget(dest, repoName string) (string, error) {
	target := filepath.Join(Expand(strings.TrimSpace(dest)), repoName)
	if _, err := os.Stat(target); err == nil {
		return target, ErrExists
	}
	return target, nil
}

// CompleteDir completes the last path segment of input against the
// directories that exist, like Tab in a shell: one match gets a trailing
// slash, several get their longest common prefix. Hidden folders only
// match when the segment starts with a dot.
func CompleteDir(input string) string {
	head, base := input, ""
	if i := strings.LastIndexAny(input, seps); i >= 0 {
		head, base = input[:i+1], input[i+1:]
	} else {
		head, base = "", input
	}
	dir := Expand(head)
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return input
	}
	var matches []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() || !strings.HasPrefix(n, base) {
			continue
		}
		if strings.HasPrefix(n, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		matches = append(matches, n)
	}
	switch len(matches) {
	case 0:
		return input
	case 1:
		return head + matches[0] + Sep
	}
	prefix := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return head + prefix
}
