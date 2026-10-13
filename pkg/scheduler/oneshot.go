package scheduler

import (
	"fmt"
	"time"
)

// OneShotCron 由一次性时间点生成对应的定时表达式（秒 分 时 日 月 *）。
// scheduledAt 优先按本地时区 "YYYY-MM-DDTHH:mm:ss" 解析，其次 RFC3339。
// 时间早于 now 或格式非法时返回错误。now 由调用方传入，便于测试。
func OneShotCron(scheduledAt string, now time.Time) (string, error) {
	t, err := time.ParseInLocation("2006-01-02T15:04:05", scheduledAt, time.Local)
	if err != nil {
		t, err = time.Parse(time.RFC3339, scheduledAt)
		if err != nil {
			return "", fmt.Errorf("无法解析一次性时间 %q：支持 YYYY-MM-DDTHH:mm:ss 或 RFC3339 格式", scheduledAt)
		}
	}
	if t.Before(now) {
		return "", fmt.Errorf("一次性时间 %q 已过，请选择未来的时间", scheduledAt)
	}
	return fmt.Sprintf("%d %d %d %d %d *",
		t.Second(), t.Minute(), t.Hour(), t.Day(), int(t.Month())), nil
}
