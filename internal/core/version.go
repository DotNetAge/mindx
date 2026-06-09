package core

// Build-time injected via LDFLAGS (see Makefile).
// Must use 'make build' to inject real values; DO NOT set defaults here.
var (
	Version   string // injected via -X, crash if empty
	Commit    = "unknown"
	BuildTime = "unknown"
	Dirty     = "clean"
)
