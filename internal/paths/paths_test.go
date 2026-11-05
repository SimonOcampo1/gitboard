package paths

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExpandAndTilde(t *testing.T) {
	h, _ := os.UserHomeDir()
	for _, in := range []string{"~/repos", "~" + Sep + "repos"} {
		if got := Expand(in); got != filepath.Join(h, "repos") {
			t.Fatalf("Expand(%q) = %q", in, got)
		}
	}
	if got := Expand("/abs"); got != "/abs" {
		t.Fatalf("Expand(/abs) = %q", got)
	}
	if got := Tilde(filepath.Join(h, "repos")); got != "~"+Sep+"repos" {
		t.Fatalf("Tilde = %q", got)
	}
}

func TestCloneTargetRefusesExisting(t *testing.T) {
	dest := t.TempDir()
	os.Mkdir(filepath.Join(dest, "taken"), 0o755)

	if _, err := CloneTarget(dest, "taken"); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	got, err := CloneTarget(dest+" ", "fresh")
	if err != nil || got != filepath.Join(dest, "fresh") {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCompleteDir(t *testing.T) {
	d := t.TempDir()
	for _, n := range []string{"repos", "repos-old", "notes", ".hidden"} {
		os.Mkdir(filepath.Join(d, n), 0o755)
	}
	os.WriteFile(filepath.Join(d, "readme.txt"), nil, 0o644)

	d += Sep
	cases := map[string]string{
		d + "no": d + "notes" + Sep, // single match -> separator
		d + "re": d + "repos",       // common prefix; the file readme.txt is ignored
		d + "zz": d + "zz",          // no match -> unchanged
		d + ".h": d + ".hidden" + Sep,
		d:        d, // notes, repos, repos-old share no prefix
	}
	for in, want := range cases {
		if got := CompleteDir(in); got != want {
			t.Errorf("CompleteDir(%q) = %q, want %q", in, got, want)
		}
	}
}
