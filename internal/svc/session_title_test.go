package svc

import "testing"

func TestSanitizeSessionTitle(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "DeepSeek 全角标记残迹整条拒绝",
			raw:  `<｜｜DSML｜｜ calls> <｜｜DSML｜｜ invoke name="`,
			want: "",
		},
		{
			name: "ASCII 特殊标记残迹整条拒绝",
			raw:  `<|tool▁calls▁begin|> invoke name="web"`,
			want: "",
		},
		{
			name: "残留标记边界拒绝",
			raw:  `部署|> 完成计划`,
			want: "",
		},
		{
			name: "正常标题透传",
			raw:  "  MindX卖点事实库建设 ",
			want: "MindX卖点事实库建设",
		},
		{
			name: "包裹引号剥离",
			raw:  `"标题内容"`,
			want: "标题内容",
		},
		{
			name: "空白折叠",
			raw:  "多段  标题\n内容",
			want: "多段 标题 内容",
		},
		{
			name: "空串",
			raw:  "   ",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeSessionTitle(tc.raw); got != tc.want {
				t.Errorf("sanitizeSessionTitle(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
