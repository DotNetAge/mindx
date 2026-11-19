package svc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/discovery"
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
	// 主机名：供远程客户端（如 iOS App 的子网扫描通道）把发现的裸 IP
	// 呈现为可读的机器名（实例名 MindX@<hostname> 与 mDNS 广播保持一致）。
	//
	// 必须用 discovery.LocalHostName 而不是 os.Hostname：macOS 上两者不同名
	// （uname 名 vs Bonjour 名），而客户端会用这个值与 mDNS 实例名做
	// 「是不是同一台机器」的匹配。两边取不同的源，地址自愈就会永远匹配失败。
	result["hostname"] = discovery.LocalHostName()

	d.logger.Info("server.version called", "result", result)

	return result, nil
}
