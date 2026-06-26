package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/DotNetAge/mindx/internal/setup"
)

func (d *Daemon) handleServerCheckUpdate(_ context.Context, params json.RawMessage) (any, error) {
	info := d.updater.Check(true)
	// Attach install source so the frontend knows how to update
	if exePath, err := os.Executable(); err == nil {
		info.InstallSource = setup.InstallSourceSlug(exePath)
	}
	return info, nil
}

func (d *Daemon) handleServerApplyUpdate(ctx context.Context, params json.RawMessage) (any, error) {
	// 通知客户端更新即将开始
	d.gw.BroadcastNotification("update_started", map[string]interface{}{
		"type": "update_started",
	})
	if err := d.updater.DownloadAndInstall(ctx); err != nil {
		return map[string]string{"error": err.Error()}, nil
	}

	// Broadcast notification to all connected clients before restart
	if d.gw != nil {
		d.gw.BroadcastNotification("update_installed", map[string]interface{}{
			"type": "update_installed",
		})
	}

	// Trigger restart in a goroutine so the RPC response can be sent back first
	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("update restart: goroutine panic", fmt.Errorf("%v", r))
			}
		}()
		// Brief delay to let the RPC response reach the client
		time.Sleep(500 * time.Millisecond)
		d.Restart()
	}()

	return map[string]string{"status": "installed", "message": "Update installed. Daemon is restarting..."}, nil
}

func (d *Daemon) handleServerRestartDaemon(ctx context.Context, params json.RawMessage) (any, error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("manual restart: goroutine panic", fmt.Errorf("%v", r))
			}
		}()
		time.Sleep(500 * time.Millisecond)
		d.Restart()
	}()
	return map[string]string{"status": "restarting", "message": "Daemon is restarting..."}, nil
}
