package core

// Build-time injected via LDFLAGS (see Makefile).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	Dirty     = "clean"
)
