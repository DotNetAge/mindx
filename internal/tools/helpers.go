package tools

// getParam 从 params map 中安全获取参数值。
func getParam(params map[string]any, key string) (any, bool) {
	if params == nil {
		return nil, false
	}
	v, ok := params[key]
	return v, ok
}
