package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileGivesDefault(t *testing.T) {
	c := Load(filepath.Join(t.TempDir(), "nope.yml"))
	if c.CloneDir != DefaultCloneDir {
		t.Fatalf("CloneDir = %q, want %q", c.CloneDir, DefaultCloneDir)
	}
}

func TestLoadBrokenFileGivesDefault(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yml")
	os.WriteFile(p, []byte(":::not yaml"), 0o644)
	if c := Load(p); c.CloneDir != DefaultCloneDir {
		t.Fatalf("CloneDir = %q, want default", c.CloneDir)
	}
}

func TestSaveThenLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "config.yml")
	if err := Save(p, Config{CloneDir: "~/code"}); err != nil {
		t.Fatal(err)
	}
	if c := Load(p); c.CloneDir != "~/code" {
		t.Fatalf("CloneDir = %q, want ~/code", c.CloneDir)
	}
}
