package bundle

import (
	"os"
	"testing"
)

// 语义比较的边界（这组用例对应真实踩到的两件事）：
//   - 雇佣/解雇会让我们自己重写 IDENTITY.md，排版全变（列表缩进、键顺序、标量引号）；
//   - 我们的写入路径写出的正文可能少一个行尾换行。
// 两者都会让「字节比较」把没改过的包判成改过，于是升级永远要确认、纳管永远认不出。

func TestSemanticDefinitionEquals(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "排版差异不算改动（缩进/键序/引号/version/hired）",
			a:    "---\nname: alpha\nrole: 画师\nskills:\n      - image\n      - archify\nicon: data:image/png;base64,AAA\nhired: true\n---\n\n正文\n",
			b:    "---\nname: alpha\nversion: 1.0.0\nrole: 画师\nskills:\n    - image\n    - archify\nicon: \"data:image/png;base64,AAA\"\n---\n\n正文\n",
			want: true,
		},
		{
			name: "正文尾部换行差异不算改动",
			a:    "---\nname: alpha\n---\n\n规则正文",
			b:    "---\nname: alpha\n---\n\n规则正文\n",
			want: true,
		},
		{
			name: "description 真改了就是改动",
			a:    "---\nname: alpha\ndescription: 旧描述\n---\n\n正文\n",
			b:    "---\nname: alpha\ndescription: 新描述\n---\n\n正文\n",
			want: false,
		},
		{
			name: "列表项真改了就是改动",
			a:    "---\nname: alpha\nskills:\n  - image\n---\n\n正文\n",
			b:    "---\nname: alpha\nskills:\n  - image\n  - archify\n---\n\n正文\n",
			want: false,
		},
		{
			name: "正文改动",
			a:    "---\nname: alpha\n---\n\n我会先画草图\n",
			b:    "---\nname: alpha\n---\n\n我直接出成品\n",
			want: false,
		},
		{
			name: "frontmatter 解析不了时按原文比（不判成相同）",
			a:    "---\nname: [未闭合\n---\n正文\n",
			b:    "---\nname: [另一个未闭合\n---\n正文\n",
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := semanticDefinitionEquals([]byte(c.a), []byte(c.b)); got != c.want {
				t.Fatalf("semanticDefinitionEquals = %v, want %v", got, c.want)
			}
		})
	}
}

// TestDefinitionMatchesIgnoresBundledSkills 包内附带的技能副本与全局库不同，
// 不影响「这个 Agent 的定义是否一致」——技能有自己的版本链。
func TestDefinitionMatchesIgnoresBundledSkills(t *testing.T) {
	root, agents, skills := newTestStores(t)
	mustWriteSkill(t, root+"/skills", "demo-skill", "演示", map[string]string{"references/a.md": "原始"})
	if err := skills.ReloadGlobal(); err != nil {
		t.Fatalf("重载失败: %v", err)
	}
	agent := mustSaveAgent(t, agents, "alpha", "规则", []string{"demo-skill"})
	pkg := root + "/alpha.mindpkg"
	if _, _, err := ExportAgent(agent, skills, pkg); err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	opts := InstallOptions{GlobalDir: root + "/skills", AgentsDir: root + "/agents"}
	same, err := DefinitionMatches(pkg, agent.Dir, opts)
	if err != nil || !same {
		t.Fatalf("刚导出的包定义应一致：same=%v err=%v", same, err)
	}

	// 技能库里的文件改了：定义仍算一致（技能归它自己的版本管）
	if err := writeFileForTest(root+"/skills/demo-skill/references/a.md", "改过的内容"); err != nil {
		t.Fatalf("改写技能失败: %v", err)
	}
	if same, _ := DefinitionMatches(pkg, agent.Dir, opts); !same {
		t.Fatal("技能副本差异不应影响 Agent 定义判定")
	}

	// 改 SOUL：才是真正的「这个 Agent 被改过」
	if err := writeFileForTest(agent.Dir+"/SOUL.md", "我自己改的规则"); err != nil {
		t.Fatalf("改写 SOUL 失败: %v", err)
	}
	if same, _ := DefinitionMatches(pkg, agent.Dir, opts); same {
		t.Fatal("SOUL 改动应判为不一致")
	}
}

// writeFileForTest 写文件（测试辅助）。
func writeFileForTest(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
