package common

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// revisionLength is the length of a revision: the commit of a pseudo-version has 12 characters.
const revisionLength = 12

// pseudoVersion matches the commit of a pseudo-version, such as
// v0.0.0-20261031165558-8e39f42dfb73, the version of `go run github.com/mincong-classroom/mc@8e39f42`.
var pseudoVersion = regexp.MustCompile(`\d{14}-([0-9a-f]{12})(\+incompatible)?$`)

// Revision returns the commit of mc that runs, in 12 characters, such as "8e39f42dfb73". It is
// "-dirty" at the end for a local build with uncommitted changes, and "unknown" when Go recorded
// no commit, e.g. for go test.
func Revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return revision(info)
}

func revision(info *debug.BuildInfo) string {
	if m := pseudoVersion.FindStringSubmatch(info.Main.Version); m != nil {
		return m[1]
	}
	var commit string
	modified := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	switch {
	case commit != "":
		commit = commit[:min(len(commit), revisionLength)]
		if modified {
			commit += "-dirty"
		}
		return commit
	case info.Main.Version != "" && info.Main.Version != "(devel)":
		return info.Main.Version // A tag, such as v1.2.0.
	}
	return "unknown"
}

// StableRevision reports whether the revision names one state of the code: a commit without
// local changes, or a tag. Two runs of a stable revision run the same code.
func StableRevision(rev string) bool {
	return rev != "unknown" && !strings.HasSuffix(rev, "-dirty")
}
