package svc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/DotNetAge/mindx/pkg/logging"
)

type WebServer struct {
	server *http.Server
	addr   string
	root   string
	logger logging.Logger
}

const DefaultWebPort = ":1313"

func NewWebServer(webDir string, logger logging.Logger) *WebServer {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &WebServer{
		addr:   DefaultWebPort,
		root:   webDir,
		logger: logger,
	}
}

func (ws *WebServer) Start(ctx context.Context) error {
	if ws.root == "" {
		return fmt.Errorf("web directory is empty")
	}

	if _, err := os.Stat(ws.root); os.IsNotExist(err) {
		ws.logger.Warn("web directory does not exist, skipping WebUI server", "dir", ws.root)
		return nil
	}

	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir(ws.root))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := filepath.Clean(r.URL.Path)

		fullPath := filepath.Join(ws.root, cleanPath)

		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			indexFile := filepath.Join(ws.root, "index.html")
			if _, statErr := os.Stat(indexFile); statErr == nil {
				http.ServeFile(w, r, indexFile)
				return
			}
			http.NotFound(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	}))

	ws.server = &http.Server{
		Addr:    ws.addr,
		Handler: mux,
	}

	go func() {
		errChan := make(chan error, 1)
		go func() {
			errChan <- ws.server.ListenAndServe()
		}()

		select {
		case <-ctx.Done():
			ws.logger.Info("shutting down WebUI server...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := ws.server.Shutdown(shutdownCtx); err != nil {
				ws.logger.Warn("WebUI server shutdown error", "error", fmt.Errorf("%w", err))
			}
			ws.logger.Info("WebUI server stopped")
		case err := <-errChan:
			if err != nil && err != http.ErrServerClosed {
				ws.logger.Error("WebUI server error", fmt.Errorf("%w", err))
			}
		}
	}()

	ws.logger.Info("WebUI server started", "addr", fmt.Sprintf("http://localhost%s", ws.addr), "root", ws.root)
	return nil
}

func (ws *WebServer) Addr() string {
	return ws.addr
}

func (ws *WebServer) URL() string {
	return fmt.Sprintf("http://localhost%s", ws.addr)
}

func WebDir(workspaceDir string) string {
	return filepath.Join(workspaceDir, "web")
}
