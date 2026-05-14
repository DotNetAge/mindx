package svc

import (
	"context"

	"github.com/DotNetAge/mindx/internal/core"
)

type Server struct {
	app    *core.App
	daemon *Daemon
}

func NewServer(addr, wsPath string) (*Server, error) {
	cfg, _ := core.LoadMindxConfig(core.DefaultUserPrefsDir())
	app, err := core.DefaultApp(cfg)
	if err != nil {
		return nil, err
	}

	daemon := NewDaemon(app, addr, wsPath)

	return &Server{
		app:    app,
		daemon: daemon,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	return s.daemon.Start(ctx)
}

func (s *Server) App() *core.App {
	return s.app
}

func (s *Server) Daemon() *Daemon {
	return s.daemon
}

func (s *Server) RegisterBuiltinCommands() {
	if s.daemon.gw == nil {
		s.daemon.initGateway()
	}
	RegisterBuiltinCommands(s.daemon.gw, s.app)
}
