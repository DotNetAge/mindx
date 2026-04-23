package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/pkg/logging"
)

// App encapsulates the MindX gateway server lifecycle.
type App struct {
	gw   *gateway.Server
	addr string
	path string

	// Boot
	logger logging.Logger
}

// NewApp creates a new App with the given listen address and WebSocket path.
func NewApp(addr, path string) *App {
	return &App{
		addr:   addr,
		path:   path,
		logger: logging.DefaultConsoleLogger(),
	}
}

// SetLogger replaces the default logger.
func (a *App) SetLogger(l logging.Logger) {
	a.logger = l
}

// DefaultHandler returns the default message handler: echo back with JSON encoding.
func (a *App) defaultHandler(msg *gateway.Message) {
	data, _ := json.Marshal(msg)
	a.gw.Send(msg.ClientID, string(data))
	slog.Info("message sent", "client", msg.ClientID)
}

// RegisterCommand adds a slash command to the gateway.
func (a *App) RegisterCommand(name string, handler gateway.CommandHandler, desc string) {
	if a.gw == nil {
		a.initGateway()
	}
	a.gw.RegisterCommand(name, handler, desc)
}

// RegisterBuiltinCommands registers the default /help, /agents, /skills commands.
func (a *App) RegisterBuiltinCommands() {
	if a.gw == nil {
		a.initGateway()
	}

	a.gw.RegisterCommand("help", func(ctx *gateway.CommandContext) (interface{}, error) {
		cmds := a.gw.CommandList()
		result := make([]string, 0, len(cmds))
		for name, desc := range cmds {
			result = append(result, fmt.Sprintf("  /%-12s %s", name, desc))
		}
		return fmt.Sprintf("可用命令:\n%s", strings.Join(result, "\n")), nil
	}, "显示所有可用命令")

	a.gw.RegisterCommand("agents", func(ctx *gateway.CommandContext) (interface{}, error) {
		// TODO: replace with actual agent listing logic
		return []map[string]string{
			{"name": "general", "description": "通用助手"},
			{"name": "coder", "description": "编程助手"},
		}, nil
	}, "列出所有可用 Agent")

	a.gw.RegisterCommand("skills", func(ctx *gateway.CommandContext) (interface{}, error) {
		// TODO: replace with actual skill listing logic
		return []map[string]string{
			{"name": "web-search", "description": "网页搜索"},
			{"name": "code-exec", "description": "代码执行"},
		}, nil
	}, "列出所有可用技能")
}

// GW returns the underlying gateway server for advanced usage.
func (a *App) GW() *gateway.Server {
	if a.gw == nil {
		a.initGateway()
	}
	return a.gw
}

// Start initializes the gateway (if not yet) and starts listening.
// It blocks until ctx is cancelled, then performs graceful shutdown.
func (a *App) Start(ctx context.Context) error {
	if a.gw == nil {
		a.initGateway()
	}

	fmt.Printf("MindX gateway starting on ws://localhost%s%s ...\n", a.addr, a.path)

	if err := a.gw.Start(); err != nil {
		return fmt.Errorf("gateway start failed: %w", err)
	}

	<-ctx.Done()
	fmt.Println("\nShutting down gateway...")
	return a.gw.Shutdown(context.Background())
}

// initGateway lazily creates the gateway server.
func (a *App) initGateway() {
	a.gw = gateway.New(
		gateway.WithAddr(a.addr),
		gateway.WithPath(a.path),
		gateway.WithHandler(a.defaultHandler),
	)
}
