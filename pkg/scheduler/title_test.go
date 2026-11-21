package scheduler

import "testing"

func TestDeriveTitle(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"首行", "每日 AI 新闻\n其余内容", "每日 AI 新闻"},
		{"跳过空行", "\n\n  周报汇总  \n", "周报汇总"},
		{"压缩空白", "  生成   周报 ", "生成 周报"},
		{"超长截断", "一二三四五六七八九十一二三四五六七八九十十一", "一二三四五六七八九十一二三四五六七八九十"},
		{"全空", "  \n\t\n", ""},
	}
	for _, c := range cases {
		if got := DeriveTitle(c.in); got != c.want {
			t.Fatalf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
