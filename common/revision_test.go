package common

import (
	"runtime/debug"
	"testing"
)

func TestRevision(t *testing.T) {
	commit := []debug.BuildSetting{{Key: "vcs.revision", Value: "8e39f42dfb73aa0011223344556677889900aabb"}}
	for _, c := range []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{
			name: "go run of a commit",
			info: debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261031165558-8e39f42dfb73"}},
			want: "8e39f42dfb73",
		},
		{
			name: "go run of a commit after a tag",
			info: debug.BuildInfo{Main: debug.Module{Version: "v1.2.1-0.20261031165558-8e39f42dfb73"}},
			want: "8e39f42dfb73",
		},
		{
			name: "local build",
			info: debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: commit},
			want: "8e39f42dfb73",
		},
		{
			name: "local build with changes",
			info: debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: append(commit, debug.BuildSetting{Key: "vcs.modified", Value: "true"}),
			},
			want: "8e39f42dfb73-dirty",
		},
		{name: "tag", info: debug.BuildInfo{Main: debug.Module{Version: "v1.2.0"}}, want: "v1.2.0"},
		{name: "go test", info: debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, want: "unknown"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := revision(&c.info); got != c.want {
				t.Errorf("revision() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestStableRevision(t *testing.T) {
	for rev, want := range map[string]bool{"8e39f42dfb73": true, "v1.2.0": true, "8e39f42dfb73-dirty": false, "unknown": false} {
		if got := StableRevision(rev); got != want {
			t.Errorf("StableRevision(%q) = %v, want %v", rev, got, want)
		}
	}
}
