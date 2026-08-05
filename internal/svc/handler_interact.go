package svc

import (
	"context"
	"encoding/json"
)

func (d *Daemon) handleMessageCancel(_ context.Context, _ json.RawMessage) (any, error) {
	d.logger.Info("message.cancel called, cancelling all running executions")
	d.clientCancels.Range(func(key, value any) bool {
		if set, ok := value.(*clientCancelSet); ok {
			set.CancelAll()
		}
		d.clientCancels.Delete(key)
		return true
	})
	return map[string]string{"status": "ok"}, nil
}
