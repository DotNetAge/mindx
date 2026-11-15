package scheduler

import "testing"

func TestValidateCronExpr_Valid(t *testing.T) {
	valid := []string{
		"0 0 9 * * *",    // 每天 9 点（6 字段秒级）
		"*/10 * * * * *", // 每 10 秒
		"0 30 8 1 * *",   // 每月 1 日 8:30
		"0 0 9 * * 1-5",  // 工作日 9 点
		"@daily",         // 描述符语法
		"0 0 9 * * MON-FRI",
	}
	for _, expr := range valid {
		if err := ValidateCronExpr(expr); err != nil {
			t.Fatalf("ValidateCronExpr(%q) = %v, want nil", expr, err)
		}
	}
}

func TestValidateCronExpr_Invalid(t *testing.T) {
	invalid := []string{
		"0 9 * * 1-5", // 5 字段：常见写法，本引擎不支持
		"",            // 空表达式
		"not a cron",
		"99 99 99 * * *", // 字段越界
	}
	for _, expr := range invalid {
		if err := ValidateCronExpr(expr); err == nil {
			t.Fatalf("ValidateCronExpr(%q) = nil, want error", expr)
		}
	}
}
