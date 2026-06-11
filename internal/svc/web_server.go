package svc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/pkg/logging"
)

type WebServer struct {
	server      *http.Server
	addr        string
	root        string
	faviconPath string
	logger      logging.Logger
	// extraHandlers stores additional HTTP handlers registered before Start().
	// They are mounted onto the mux at startup, taking precedence over the
	// catch-all "/" SPA handler.
	extraHandlers []handlerRegistration
}

type handlerRegistration struct {
	pattern string
	handler http.HandlerFunc
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

// SetFavicon sets an optional external favicon file path.
// When set, /favicon.ico will serve this file even if not present in webDir.
func (ws *WebServer) SetFavicon(path string) {
	ws.faviconPath = path
}

// HandleFunc registers an additional HTTP handler to be mounted when the
// server starts. Patterns are matched in the order they are registered and
// take precedence over the default SPA catch-all ("/") handler.
//
// Must be called before Start().
func (ws *WebServer) HandleFunc(pattern string, handler http.HandlerFunc) {
	ws.extraHandlers = append(ws.extraHandlers, handlerRegistration{
		pattern: pattern,
		handler: handler,
	})
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

	// Mount extra API handlers before the SPA catch-all so they take
	// precedence on matched paths (e.g. /api/health, /api/log/download).
	for _, h := range ws.extraHandlers {
		mux.HandleFunc(h.pattern, h.handler)
		ws.logger.Info("web server: registered handler", "pattern", h.pattern)
	}

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

	// Favicon: serve from configured icon path or fallback to webDir
	mux.HandleFunc("/favicon.ico", ws.handleFavicon)
	mux.HandleFunc("/favicon.png", ws.handleFavicon)

	// API: 下载日志文件
	// GET /api/log/download?stream=main|error
	// 返回 Content-Disposition: attachment，浏览器原生下载
	mux.HandleFunc("/api/log/download", ws.handleLogDownload)

	// 先尝试监听端口，如果端口被占用立即返回错误
	listener, err := net.Listen("tcp", ws.addr)
	if err != nil {
		return fmt.Errorf("web server listen on %s: %w", ws.addr, err)
	}

	ws.server = &http.Server{
		Addr:    ws.addr,
		Handler: mux,
	}

	go func() {
		// 使用 listener 的 Serve（同步阻塞），监听错误由 ctx 管理
		if serveErr := ws.server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			ws.logger.Error("WebUI server error", fmt.Errorf("%w", serveErr))
		}
	}()

	go func() {
		<-ctx.Done()
		ws.logger.Info("shutting down WebUI server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ws.server.Shutdown(shutdownCtx); err != nil {
			ws.logger.Warn("WebUI server shutdown error", "error", fmt.Errorf("%w", err))
		}
		ws.logger.Info("WebUI server stopped")
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

// handleFavicon serves the app icon as favicon.
// Priority: configured iconPath → webDir/favicon.ico → webDir/favicon.png → 404
func (ws *WebServer) handleFavicon(w http.ResponseWriter, r *http.Request) {
	// Try configured external icon path first
	if ws.faviconPath != "" {
		if info, err := os.Stat(ws.faviconPath); err == nil && !info.IsDir() {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, ws.faviconPath)
			return
		}
	}

	// Fallback: look in webDir
	candidates := []string{"favicon.ico", "favicon.png"}
	for _, name := range candidates {
		candidate := filepath.Join(ws.root, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, candidate)
			return
		}
	}

	http.NotFound(w, r)
}

// handleLogDownload 下载日志文件
//
//	GET /api/log/download?stream=main|error
//
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
