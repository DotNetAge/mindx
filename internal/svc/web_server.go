package svc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	// API: 下载日志文件
	// GET /api/log/download?stream=main|error
	// 返回 Content-Disposition: attachment，浏览器原生下载
	mux.HandleFunc("/api/log/download", ws.handleLogDownload)

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

// handleLogDownload 下载日志文件
//   GET /api/log/download?stream=main|error
// 通过白名单选择文件名，禁止路径穿越
func (ws *WebServer) handleLogDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stream := r.URL.Query().Get("stream")
	if stream == "" {
		stream = "main"
	}
	filename, ok := logStreamFilenames[stream]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown stream %q (allowed: main, error)", stream), http.StatusBadRequest)
		return
	}

	logPath := filepath.Join(logging.ResolveLogDir(), filename)
	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在 — 返回 204 避免前端报错
			w.WriteHeader(http.StatusNoContent)
			return
		}
		ws.logger.Error("log download stat failed", err, "path", logPath)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 防御性：确保最终路径仍在 logDir 内（防止未来扩参数时被绕过）
	cleanLogPath := filepath.Clean(logPath)
	if !strings.HasPrefix(cleanLogPath, filepath.Clean(logging.ResolveLogDir())+string(os.PathSeparator)) &&
		cleanLogPath != filepath.Clean(logging.ResolveLogDir()) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, cleanLogPath)
}
