// Package config persists the few settings gitboard remembers between runs.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefaultCloneDir = "~/repos"

type Config struct {
	// CloneDir is the last Clone destination used; the next clone prompt starts there.
	CloneDir string `yaml:"clone_dir"`
}

// Path is ~/.config/gitboard/config.yml (or the platform equivalent).
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "gitboard", "config.yml")
}

// Load never fails: a missing or broken file yields the defaults.
func Load(path string) Config {
	c := Config{}
	if b, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(b, &c)
	}
	if c.CloneDir == "" {
		c.CloneDir = DefaultCloneDir
	}
	return c
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
