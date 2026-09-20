// Package version exposes build metadata injected at link time via -ldflags.
package version

import "fmt"

var (
	// Version is the semantic version, set with
	// -ldflags "-X github.com/medcelerate/disguise-mcp/internal/version.Version=v0.1.0".
	Version = "dev"
	// Commit is the git commit hash.
	Commit = "none"
	// Date is the build date.
	Date = "unknown"
)

// String returns a human-readable version line.
func String() string {
	return fmt.Sprintf("disguise-mcp %s (commit %s, built %s)", Version, Commit, Date)
}
