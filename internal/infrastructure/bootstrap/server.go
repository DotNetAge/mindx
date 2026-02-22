package bootstrap

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"mindx/internal/adapters/http/middleware"
)

// Server HTTP API 服务器
// 职责: 提供 HTTP API 和静态文件服务
type Server struct {
	engine         *gin.Engine        // Gin 引擎
	httpServer     *http.Server       // HTTP 服务器
	staticDir      string             // 静态文件目录
	port           int                // 服务器端口
	shutdownCtx    context.Context    // 关闭上下文
	shutdownCancel context.CancelFunc // 关闭取消函数
}

// NewServer 创建 HTTP API 服务器实例
func NewServer(port int, staticDir string) (*Server, error) {
	// 默认端口 1314
	if port <= 0 {
		port = 1314
	}

	// 创建关闭上下文
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	// 创建 Gin 引擎
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(gin.Logger())
	engine.Use(middleware.RequestID())
	engine.Use(middleware.MetricsMiddleware())

	return &Server{
		engine:         engine,
		port:           port,
		staticDir:      staticDir,
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	// 设置静态文件服务
	if err := s.setupStaticFiles(); err != nil {
		log.Printf("警告: %v", err)
	}

	// 注册健康检查路由
	s.engine.GET("/health", s.handleHealthCheck)
	s.engine.GET("/ready", s.handleReadyCheck)

	// 注册 Prometheus 指标端点
	s.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 创建 HTTP 服务器
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.engine,
	}

	// 在 goroutine 中启动服务器
	go func() {
		log.Printf("HTTP API 服务器启动中 http://localhost:%d", s.port)
		if s.staticDir != "" {
			log.Printf("提供静态文件服务: %s", s.staticDir)
		}
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("服务器错误: %v", err)
		}
	}()

	return nil
}

// Stop 停止服务器(非优雅关闭)
func (s *Server) Stop() error {
	return s.Shutdown(0)
}

// Shutdown 优雅关闭服务器
// timeout: 超时时间(秒),0 表示立即关闭
func (s *Server) Shutdown(timeout int) error {
	log.Println("正在关闭 HTTP API 服务器...")

	// 取消关闭上下文
	if s.shutdownCancel != nil {
		s.shutdownCancel()
	}

	// 关闭 HTTP 服务器
	if s.httpServer != nil {
		var ctx context.Context
		var cancel context.CancelFunc

		if timeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			defer cancel()
		} else {
			ctx = context.Background()
		}

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP 服务器关闭错误: %v", err)
			return err
		}
		log.Println("HTTP 服务器已关闭")
	}

	log.Println("服务器关闭完成")
	return nil
}

// setupStaticFiles 设置静态文件服务
func (s *Server) setupStaticFiles() error {
	staticDir, err := s.getStaticDir()
	if err != nil {
		return err
	}
	s.staticDir = staticDir

	// 提供静态文件服务（使用 /static 路径避免与 /api 冲突）
	s.engine.Static("/static", staticDir)
	s.engine.StaticFile("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))
	return nil
}

// getStaticDir 获取静态文件目录
func (s *Server) getStaticDir() (string, error) {
	if _, err := os.Stat(s.staticDir); err == nil {
		return s.staticDir, nil
	}
	return "", fmt.Errorf("static directory not found")
}

// handleHealthCheck 健康检查端点
func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

// handleReadyCheck 就绪检查端点
func (s *Server) handleReadyCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ready":     true,
		"timestamp": time.Now().Unix(),
	})
}

// GetEngine 获取 Gin 引擎实例(用于添加自定义路由)
func (s *Server) GetEngine() *gin.Engine {
	return s.engine
}

// GracefulShutdown 优雅关闭(默认超时10秒)
func (s *Server) GracefulShutdown() {
	_ = s.Shutdown(10) // 关闭失败不阻塞
}
