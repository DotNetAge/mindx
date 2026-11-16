package bundle

import (
	"strings"
	"testing"
)

// TestResolvePublishVersion 版本权威收敛：frontmatter 是事实源，参数只能缺省或一致。
func TestResolvePublishVersion(t *testing.T) {
	cases := []struct {
		name        string
		kind        PackageKind
		param       string
		frontmatter string
		want        string
		wantErr     string
	}{
		{
			name:        "只有 frontmatter：取它",
			kind:        KindAgent,
			frontmatter: "1.2.0",
			want:        "1.2.0",
		},
		{
			name:  "只有参数：取参数（尚未写 version 的实体仍可发布）",
			kind:  KindAgent,
			param: "1.0.1",
			want:  "1.0.1",
		},
		{
			name:        "两者一致：放行",
			kind:        KindTeam,
			param:       "2.0.0",
			frontmatter: "2.0.0",
			want:        "2.0.0",
		},
		{
			name:        "两者不一致：拒绝（防第二事实源）",
			kind:        KindAgent,
			param:       "1.0.0",
			frontmatter: "1.3.0",
			wantErr:     "两个说法",
		},
		{
			name:    "都空：拒绝并指引去写 frontmatter",
			kind:    KindAgent,
			wantErr: "IDENTITY.md",
		},
		{
			name:    "团队都空：报错点名 TEAM.md",
			kind:    KindTeam,
			wantErr: "TEAM.md",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolvePublishVersion(c.kind, "demo", c.param, c.frontmatter)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("应报错，实际得到 %q", got)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("错误文案未点名 %q：%v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("不应报错：%v", err)
			}
			if got != c.want {
				t.Fatalf("版本 = %q, want %q", got, c.want)
			}
		})
	}
}
