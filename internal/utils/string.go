package utils

import "strings"

// ToSnakeCase 将驼峰命名转换为蛇形命名
// 例如: MyVariable -> my_variable
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	for i, c := range s {
		if i > 0 && (c >= 'A' && c <= 'Z') {
			result.WriteByte('_')
		}
		result.WriteRune(c)
	}
	return strings.ToLower(result.String())
}

// ToCamelCase 将蛇形命名转换为驼峰命名
// 例如: my_variable -> MyVariable
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}

	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// ContainsString 检查字符串是否包含切片中的任意一个字符串
func ContainsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// ContainsStringCaseInsensitive 检查字符串是否包含切片中的任意一个字符串（不区分大小写）
func ContainsStringCaseInsensitive(slice []string, target string) bool {
	targetLower := strings.ToLower(target)
	for _, s := range slice {
		if strings.ToLower(s) == targetLower {
			return true
		}
	}
	return false
}

// RemoveDuplicates 移除字符串切片中的重复项
func RemoveDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// TrimStringSlice 从字符串切片的两端移除指定字符
func TrimStringSlice(slice []string, cutset string) []string {
	result := make([]string, len(slice))
	for i, s := range slice {
		result[i] = strings.Trim(s, cutset)
	}
	return result
}
