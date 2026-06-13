package svc

import (
	"context"
	"encoding/json"
	"time"
)

func (d *Daemon) handleServerCheckUpdate(_ context.Context, params json.RawMessage) (any, error) {
	info := d.updater.Check(true)
	return info, nil
}

func (d *Daemon) handleServerApplyUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	if err := d.updater.DownloadAndInstall(ctx); err != nil {
		return map[string]string{"error": err.Error()}, nil
	}

	// Trigger restart in a goroutine so the RPC response can be sent back first
	go func() {
		// Brief delay to let the RPC response reach the client
		time.Sleep(500 * time.Millisecond)
		d.Restart()
	}()

	return map[string]string{"status": "installed", "message": "Update installed. Daemon is restarting..."}, nil
}

func (d *Daemon) handleServerRestartDaemon(ctx context.Context, params json.RawMessage) (any, error) {
	go func() {
		time.Sleep(500 * time.Millisecond)
		d.Restart()
	}()
	return map[string]string{"status": "restarting", "message": "Daemon is restarting..."}, nil
}
