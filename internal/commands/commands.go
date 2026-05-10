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

// RegisterAll registers all commands from the registry to the gateway server.
func (r *Registry) RegisterAll(gw *gateway.Server) {
	for _, c := range r.commands {
		gw.RegisterCommand(c.meta, c.handler)
	}
}

// New creates a new Registry and populates it with all built-in commands.
func New() *Registry {
	r := &Registry{}
	registerSystemCommands(r)
	registerCatalogCommands(r)
	registerSchedulerCommands(r)
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
