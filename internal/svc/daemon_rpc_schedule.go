package svc

import (
	"context"
	"encoding/json"
	"fmt"
)

func (d *Daemon) handleScheduleList(_ context.Context, _ json.RawMessage) (any, error) {
	if d.schedulerDB == nil {
		return nil, fmt.Errorf("scheduler not available")
	}

	entries, err := d.schedulerDB.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list schedules failed: %w", err)
	}

	if entries == nil {
		return []any{}, nil
	}

	return entries, nil
}
