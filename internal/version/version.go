// Package version carries build metadata. Values are overridden at release
// time via -ldflags -X (see .goreleaser.yaml); the defaults are what a
// `go build`/`go run` from source reports.
package version

import "fmt"

var (
	Version = "dev"     // semver tag, e.g. v0.2.0
	Commit  = "none"    // short git SHA
	Date    = "unknown" // RFC3339 build date
)

// String is the human one-liner (root --version).
func String() string {
	return fmt.Sprintf("boursocli %s (commit %s, built %s)", Version, Commit, Date)
}

// Info is the agent-first JSON shape (the `version` command).
func Info() map[string]string {
	return map[string]string{"version": Version, "commit": Commit, "date": Date}
}
