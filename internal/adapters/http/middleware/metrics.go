package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mindx_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	HttpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mindx_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	LlmCallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mindx_llm_calls_total",
		Help: "Total number of LLM calls",
	}, []string{"model", "status"})

	LlmCallDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mindx_llm_call_duration_seconds",
		Help:    "LLM call duration in seconds",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
	}, []string{"model"})

	TokenUsageTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mindx_token_usage_total",
		Help: "Total token usage",
	}, []string{"model", "type"})

	ChannelMessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mindx_channel_messages_total",
		Help: "Total channel messages",
	}, []string{"channel", "direction"})

	ActiveWsConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mindx_active_ws_connections",
		Help: "Number of active WebSocket connections",
	})
)

// MetricsMiddleware Prometheus HTTP 指标中间件
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		HttpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		HttpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
