package scheduler

import (
	"fmt"

	"github.com/robfig/cron/v3"
)

// ValidateCronExpr 校验表达式能否被调度引擎解析。
// 引擎以秒级 6 字段（秒 分 时 日 月 周）运行，5 字段的常见写法
// （如 "0 9 * * 1-5"）在此会解析失败——创建入口必须先行校验，
// 避免任务保存成功却永不触发的静默失效。
func ValidateCronExpr(expr string) error {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour |
		cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(expr); err != nil {
		return fmt.Errorf("cron 表达式无法解析：%v（本系统使用 6 字段秒级格式：秒 分 时 日 月 周，如 0 0 9 * * * 表示每天 9 点）", err)
	}
	return nil
}
