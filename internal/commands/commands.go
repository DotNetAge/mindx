package commands

import "github.com/DotNetAge/gort/pkg/gateway"

// Meta is an alias for gateway.CommandMeta, the unified command metadata model.
type Meta = gateway.CommandMeta

// Handler is the server-side command handler signature.
type Handler func(ctx *gateway.CommandContext) (any, error)

// Registry holds all command metadata and handlers in one place.
type Registry struct {
	commands []struct {
		meta    Meta
		handler Handler
	}
}

// Register adds a command with its metadata and handler.
func (r *Registry) Register(meta Meta, handler Handler) {
	r.commands = append(r.commands, struct {
		meta    Meta
		handler Handler
	}{meta, handler})
}

// Metas returns all command metadata (for client sync).
func (r *Registry) Metas() []Meta {
	out := make([]Meta, 0, len(r.commands))
	for _, c := range r.commands {
		out = append(out, c.meta)
	}
	return out
}

// Wrap 是命令注册的横切包装器：调用方据此把鉴权等统一闸门施加到每条命令上。
type Wrap func(name string, h Handler) Handler

// Registerer 是命令注册动作（生产环境传 gw.RegisterCommand，测试可换成记录器）。
type Registerer func(meta Meta, handler func(ctx *gateway.CommandContext) (any, error))

// RegisterAllWith 按给定注册动作逐条注册命令，并允许对每条命令施加 wrap。
//
// 这是命令注册的唯一入口，刻意没有提供「不带 wrap 直接注册」的便利方法：
// RegisterCommand 会把命令直接写进 gateway 的 method 表，与 RegisterMethod
// 是两条不同的注册路径，任何只挂在 RegisterMethod 上的横切逻辑（设备接入闸、
// 作用域闸）都不会覆盖命令，于是「注册路径不同 = 鉴权不同」。留下一条免闸
// 注册的捷径，等于把这道缺口重新打开一次。
func (r *Registry) RegisterAllWith(register Registerer, wrap Wrap) {
	if register == nil {
		return
	}
	for _, c := range r.commands {
		h := c.handler
		if wrap != nil {
			h = wrap(c.meta.Name, h)
		}
		register(c.meta, h)
	}
}

// New creates a new Registry and populates it with all built-in commands.
func New() *Registry {
	r := &Registry{}
	registerSystemCommands(r)
	registerCatalogCommands(r)
	registerSchedulerCommands(r)
	registerCompactCommands(r)
	return r
}

// LocalRegistry holds client-only commands (no server handler).
type LocalRegistry struct {
	commands []struct {
		meta Meta
	}
}

// Register adds a local-only command.
func (r *LocalRegistry) Register(meta Meta) {
	r.commands = append(r.commands, struct {
		meta Meta
	}{meta})
}

// Metas returns all local command metadata.
func (r *LocalRegistry) Metas() []Meta {
	out := make([]Meta, 0, len(r.commands))
	for _, c := range r.commands {
		out = append(out, c.meta)
	}
	return out
}

// NewLocal creates a registry for client-only commands.
func NewLocal() *LocalRegistry {
	r := &LocalRegistry{}
	registerLocalCommands(r)
	return r
}
