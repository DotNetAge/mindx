package handlers

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type MonitorHandler struct {
	logsDir string
}

type LogEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Logger    string         `json:"logger,omitempty"`
	Caller    string         `json:"caller,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

type ZapLogEntry struct {
	Time       string `json:"time"`
	Level      string `json:"level"`
	LoggerName string `json:"logger,omitempty"`
	Caller     string `json:"caller,omitempty"`
	Msg        string `json:"msg"`
	Stacktrace string `json:"stacktrace,omitempty"`
}

func NewMonitorHandler() *MonitorHandler {
	return &MonitorHandler{
		logsDir: "logs",
	}
}

func (h *MonitorHandler) getLogs(c *gin.Context) {
	level := c.Query("level")
	limit := c.DefaultQuery("limit", "100")
	since := c.Query("since")

	logPath := filepath.Join(h.logsDir, "system.log")
	entries, err := h.readLogLines(logPath, level, limit, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "读取日志失败: " + err.Error(),
		})
		return
	}

	lastTimestamp := ""
	if len(entries) > 0 {
		lastTimestamp = entries[0].Timestamp
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":          entries,
		"lastTimestamp": lastTimestamp,
		"count":         len(entries),
	})
}

func (h *MonitorHandler) clearLogs(c *gin.Context) {
	logPath := filepath.Join(h.logsDir, "system.log")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "日志文件不存在，无需清空",
		})
		return
	}

	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "清空日志失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志已清空",
	})
}

// readLogLines 读取并解析日志文件（支持增量）
func (h *MonitorHandler) readLogLines(filePath string, filterLevel string, limitStr string, since string) ([]LogEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEntry{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var entries []LogEntry
	var allEntries []LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		entry, err := h.parseLogLine(line)
		if err != nil {
			continue
		}

		// 过滤日志级别
		if filterLevel != "" && !strings.EqualFold(entry.Level, filterLevel) {
			continue
		}

		// 如果指定了 since，只返回该时间戳之后的新日志
		if since != "" && entry.Timestamp <= since {
			continue
		}

		allEntries = append(allEntries, entry)
	}

	// 如果是增量查询（指定了 since），按时间正序返回
	if since != "" {
		entries = allEntries
	} else {
		// 倒序（最新的在前面）
		for i := len(allEntries) - 1; i >= 0; i-- {
			entries = append(entries, allEntries[i])
		}

		// 限制返回数量
		limit := 100
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
		if len(entries) > limit {
			entries = entries[:limit]
		}
	}

	return entries, nil
}

// parseLogLine 解析单行日志(Zap JSON格式)
func (h *MonitorHandler) parseLogLine(line string) (LogEntry, error) {
	var zapLog ZapLogEntry
	if err := json.Unmarshal([]byte(line), &zapLog); err != nil {
		return LogEntry{}, err
	}

	// 提取额外字段
	extra := make(map[string]any)
	var genericMap map[string]any
	if err := json.Unmarshal([]byte(line), &genericMap); err == nil {
		for k, v := range genericMap {
			if k != "time" && k != "level" && k != "logger" && k != "caller" && k != "msg" && k != "stacktrace" {
				extra[k] = v
			}
		}
	}

	return LogEntry{
		Timestamp: zapLog.Time,
		Level:     zapLog.Level,
		Message:   zapLog.Msg,
		Logger:    zapLog.LoggerName,
		Caller:    zapLog.Caller,
		Extra:     extra,
	}, nil
}
