package main

import (
	"fmt"
	"runtime"
)

// These variables are populated at build time via -ldflags.
// Defaults are used when the project is built without ldflags,
// e.g. during local development with plain `go build`.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// versionString returns a single-line version description suitable
// for printing to the console and writing to the log.
func versionString() string {
	return fmt.Sprintf(
		"%s (commit %s, built %s, %s)",
		Version, Commit, BuildDate, runtime.Version(),
	)
}

// shortVersion returns just the semantic version, without build metadata.
// Useful for Prometheus labels where high cardinality should be avoided.
func shortVersion() string {
	return Version
}
