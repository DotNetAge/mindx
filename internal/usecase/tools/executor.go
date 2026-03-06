package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"mindx/pkg/logging"
)

// Executor 工具执行器
type Executor struct {
	logger logging.Logger
}

// NewExecutor 创建执行器
func NewExecutor() *Executor {
	return &Executor{
		logger: logging.GetSystemLogger().Named("tool_executor"),
	}
}

// Execute 执行工具
func (e *Executor) Execute(tool *Tool, params map[string]interface{}) (string, error) {
	e.logger.Debug("executing tool",
		logging.String("name", tool.Name),
		logging.String("type", tool.Type),
	)

	// 根据工具类型执行
	switch tool.Type {
	case "go":
		return e.executeGo(tool, params)
	case "python":
		return e.executePython(tool, params)
	case "shell":
		return e.executeShell(tool, params)
	case "builtin":
		return e.executeBuiltin(tool, params)
	default:
		return "", fmt.Errorf("unsupported tool type: %s", tool.Type)
	}
}

// executeGo 执行 Go 工具
func (e *Executor) executeGo(tool *Tool, params map[string]interface{}) (string, error) {
	// 序列化参数为 JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal params: %w", err)
	}

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(tool.Timeout)*time.Second)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(ctx, tool.Command)
	cmd.Dir = tool.WorkDir
	cmd.Stdin = bytes.NewReader(paramsJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		e.logger.Error("tool execution failed",
			logging.String("tool", tool.Name),
			logging.String("stderr", stderr.String()),
			logging.Err(err),
		)
		return "", fmt.Errorf("execution failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// executePython 执行 Python 工具
func (e *Executor) executePython(tool *Tool, params map[string]interface{}) (string, error) {
	// 序列化参数为 JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal params: %w", err)
	}

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(tool.Timeout)*time.Second)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(ctx, "python3", tool.Command)
	cmd.Dir = tool.WorkDir
	cmd.Stdin = bytes.NewReader(paramsJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		e.logger.Error("tool execution failed",
			logging.String("tool", tool.Name),
			logging.String("stderr", stderr.String()),
			logging.Err(err),
		)
		return "", fmt.Errorf("execution failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// executeShell 执行 Shell 工具
func (e *Executor) executeShell(tool *Tool, params map[string]interface{}) (string, error) {
	// 序列化参数为 JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal params: %w", err)
	}

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(tool.Timeout)*time.Second)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(ctx, "sh", tool.Command)
	cmd.Dir = tool.WorkDir
	cmd.Stdin = bytes.NewReader(paramsJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		e.logger.Error("tool execution failed",
			logging.String("tool", tool.Name),
			logging.String("stderr", stderr.String()),
			logging.Err(err),
		)
		return "", fmt.Errorf("execution failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// executeBuiltin 执行内置工具
func (e *Executor) executeBuiltin(tool *Tool, params map[string]interface{}) (string, error) {
	// 内置工具通过注册的函数执行
	// TODO: 实现内置工具注册机制
	return "", fmt.Errorf("builtin tools not implemented yet")
}

// ExecuteTool 执行工具（ToolManager 的方法）
func (tm *ToolManager) ExecuteTool(name string, params map[string]interface{}) (string, error) {
	// 获取工具
	tool, err := tm.GetTool(name)
	if err != nil {
		return "", err
	}

	// 创建执行器
	executor := NewExecutor()

	// 执行工具
	result, err := executor.Execute(tool, params)
	if err != nil {
		tm.logger.Error("tool execution failed",
			logging.String("tool", name),
			logging.Err(err),
		)
		return "", err
	}

	tm.logger.Info("tool executed successfully",
		logging.String("tool", name),
	)

	return result, nil
}
