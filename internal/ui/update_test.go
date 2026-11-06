package ui

import (
	"runtime/debug"
	"testing"
)

func TestRevision(t *testing.T) {
	set := func(kv ...string) []debug.BuildSetting {
		var s []debug.BuildSetting
		for i := 0; i < len(kv); i += 2 {
			s = append(s, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
		}
		return s
	}
	for _, c := range []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{"go install @latest", debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261105224101-96b2a1b83184"}}, "96b2a1b83184"},
		{"clone build", debug.BuildInfo{Settings: set("vcs.revision", "2cf72f3abc", "vcs.modified", "false")}, "2cf72f3abc"},
		{"dirty clone", debug.BuildInfo{Settings: set("vcs.revision", "2cf72f3abc", "vcs.modified", "true")}, ""},
		{"go run", debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ""},
		{"tagged", debug.BuildInfo{Main: debug.Module{Version: "v1.2.0"}}, ""},
	} {
		if got := revision(&c.info); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
