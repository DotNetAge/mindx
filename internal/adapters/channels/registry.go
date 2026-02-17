package channels

import (
	"fmt"

	"mindx/internal/core"
)

// ChannelFactory Channel 工厂函数类型
// 参数: cfg - Channel 的配置 map
// 返回: Channel 实例，error - 创建失败时返回错误
type ChannelFactory func(cfg map[string]interface{}) (core.Channel, error)

// ChannelRegistry Channel 注册中心
// 负责管理所有 Channel 的工厂函数，实现配置驱动的 Channel 创建
type ChannelRegistry struct {
	factories map[string]ChannelFactory
}

// globalRegistry 全局注册中心实例
var globalRegistry = &ChannelRegistry{
	factories: make(map[string]ChannelFactory),
}

// Register 注册 Channel 工厂函数
// 各个 Channel 包的 init() 函数中调用此方法自动注册
func Register(name string, factory ChannelFactory) {
	if factory == nil {
		panic(fmt.Sprintf("Channel %s: factory function cannot be nil", name))
	}
	globalRegistry.factories[name] = factory
}

// GetFactory 获取指定 Channel 的工厂函数
func GetFactory(name string) (ChannelFactory, bool) {
	factory, ok := globalRegistry.factories[name]
	return factory, ok
}

// ListFactories 列出所有已注册的 Channel 工厂名称
func ListFactories() []string {
	names := make([]string, 0, len(globalRegistry.factories))
	for name := range globalRegistry.factories {
		names = append(names, name)
	}
	return names
}

// IsRegistered 检查 Channel 是否已注册
func IsRegistered(name string) bool {
	_, ok := globalRegistry.factories[name]
	return ok
}

// getStringFromConfig 从配置 map 中获取字符串值
// 辅助函数，用于各个 Channel 的工厂函数
// 如果配置不存在或类型不匹配，返回空字符串
func getStringFromConfig(cfg map[string]interface{}, key string) string {
	if cfg == nil {
		return ""
	}
	val, ok := cfg[key]
	if !ok {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

// getStringFromConfigWithDefault 从配置 map 中获取字符串值，支持默认值
// 辅助函数，用于各个 Channel 的工厂函数
// 如果配置不存在或类型不匹配，返回 defaultValue
func getStringFromConfigWithDefault(cfg map[string]interface{}, key, defaultValue string) string {
	if cfg == nil {
		return defaultValue
	}
	val, ok := cfg[key]
	if !ok {
		return defaultValue
	}
	if str, ok := val.(string); ok {
		return str
	}
	return defaultValue
}

// getIntFromConfig 从配置 map 中获取整数值
// 辅助函数，用于各个 Channel 的工厂函数
// 如果配置不存在或类型不匹配，返回 defaultValue
func getIntFromConfig(cfg map[string]interface{}, key string, defaultValue int) int {
	if cfg == nil {
		return defaultValue
	}
	val, ok := cfg[key]
	if !ok {
		return defaultValue
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		// 尝试将字符串转换为整数
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}

// getBoolFromConfig 从配置 map 中获取布尔值
// 辅助函数，用于各个 Channel 的工厂函数
// 如果配置不存在或类型不匹配，返回 defaultValue
func getBoolFromConfig(cfg map[string]interface{}, key string, defaultValue bool) bool {
	if cfg == nil {
		return defaultValue
	}
	val, ok := cfg[key]
	if !ok {
		return defaultValue
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	case int, float64:
		var intVal int
		if i, ok := v.(int); ok {
			intVal = i
		} else {
			intVal = int(v.(float64))
		}
		return intVal != 0
	}
	return defaultValue
}
