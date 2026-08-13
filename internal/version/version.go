// Package version carries the build metadata goreleaser injects through
// -ldflags. The defaults are what a plain `go build` produces.
package version

import "runtime/debug"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String is the one-line version used in logs, the API and the User-Agent.
func String() string { return Version + " (" + Commit + ", " + Date + ")" }

// UserAgent identifies Snagarr to every service it calls.
func UserAgent() string { return "snagarr/" + Version }

func init() {
	if Version != "dev" {
		return
	}
	// `go install module@version` records the version even though ldflags never
	// ran, so prefer it over the placeholder.
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}
