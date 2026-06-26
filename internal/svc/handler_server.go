package svc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/DotNetAge/mindx/internal/core"
)

var errVersionNotSet = errors.New("binary was not built with a version tag; use 'make build' to inject the version")

func (d *Daemon) handleServerVersion(_ context.Context, params json.RawMessage) (any, error) {
	// Reject binaries built without proper version tag (use 'make build' instead of raw 'go build')
	if core.Version == "" {
		return nil, errVersionNotSet
	}

	result := map[string]string{
		"version":    core.Version,
		"commit":     core.Commit,
		"build_time": core.BuildTime,
	}

	d.logger.Info("server.version called", "result", result)

	return result, nil
}
