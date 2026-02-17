package utils

import (
	"fmt"
	"time"
)

// Timezone 常用时区
const (
	TimezoneUTC   = "UTC"
	TimezoneAsia  = "Asia/Shanghai"
)

// NowInTimezone 获取指定时区的当前时间
func NowInTimezone(timezone string) time.Time {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Now()
	}
	return time.Now().In(loc)
}

// FormatTime 格式化时间为字符串
func FormatTime(t time.Time, format string) string {
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	return t.Format(format)
}

// ParseTime 解析时间字符串
func ParseTime(timeStr, format string) (time.Time, error) {
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	return time.Parse(format, timeStr)
}

// FormatDuration 格式化时间持续时间为人类可读的字符串
// 例如: 1h30m5s -> "1小时30分钟5秒"
func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	var result string
	if h > 0 {
		result += fmt.Sprintf("%d小时", h)
	}
	if m > 0 {
		result += fmt.Sprintf("%d分钟", m)
	}
	if s > 0 {
		result += fmt.Sprintf("%d秒", s)
	}

	if result == "" {
		return "0秒"
	}
	return result
}

// IsExpired 检查时间是否已过期
func IsExpired(t time.Time) bool {
	return time.Now().After(t)
}

// TimeUntil 计算从现在到指定时间的时间差
func TimeUntil(t time.Time) time.Duration {
	return t.Sub(time.Now())
}

// TimeSince 计算从指定时间到现在的时间差
func TimeSince(t time.Time) time.Duration {
	return time.Since(t)
}

// GetStartOfDay 获取一天的起始时间
func GetStartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// GetEndOfDay 获取一天的结束时间
func GetEndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}

// GetStartOfMonth 获取月份的起始时间
func GetStartOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// GetEndOfMonth 获取月份的结束时间
func GetEndOfMonth(t time.Time) time.Time {
	return t.AddDate(0, 1, -1).AddDate(0, 0, 1).Add(-time.Second)
}
