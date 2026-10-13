package scheduler

import (
	"testing"
	"time"
)

func TestOneShotCron(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.Local)
	future := time.Date(2026, 10, 13, 14, 30, 5, 0, time.Local)

	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"本地时区格式", "2026-10-13T14:30:05", "5 30 14 13 10 *", false},
		{"RFC3339 格式", future.Format(time.RFC3339), "5 30 14 13 10 *", false},
		{"过去时间报错", "2026-10-13T09:00:00", "", true},
		{"非法格式报错", "明天下午三点", "", true},
		{"空串报错", "", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := OneShotCron(tc.in, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("OneShotCron(%q) 期望报错，实际得到 %q", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("OneShotCron(%q) 意外报错：%v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("OneShotCron(%q) = %q，期望 %q", tc.in, got, tc.want)
			}
		})
	}
}
