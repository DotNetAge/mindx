package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/goharness/logging"
	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

// 冒烟验证（评审问题 2）：基于文件的 SOUL 段不再注入外部标题，
// 标题由 SOUL.md 自身处理；SOUL 为空时整段跳过（无标题空悬）。
func TestSoulSectionNoInjectedTitle(t *testing.T) {
	testDeadline(t)

	tmp := t.TempDir()
	skills, err := skillstore.NewStore(filepath.Join(tmp, "skills"), filepath.Join(tmp, "agents"))
	if err != nil {
		t.Fatalf("构造技能库失败: %v", err)
	}
	agents, _, loadErr := agentstore.Load(filepath.Join(tmp, "agents"))
	if loadErr != nil {
		t.Fatalf("构造 Agent 库失败: %v", loadErr)
	}
	if err := agents.Save(&agentstore.Agent{
		Meta: agentstore.AgentMeta{Name: "tester", Role: "冒烟"},
		Soul: "## 核心逻辑\n\n由 SOUL.md 自带的标题",
	}); err != nil {
		t.Fatalf("保存 Agent 失败: %v", err)
	}

	pb := NewPromptBuilder(agents, skills, "/tmp/prefs", "/tmp/venv", nil)

	store, err := mindxses.NewFileSessionStore(filepath.Join(tmp, "sessions"))
	if err != nil {
		t.Fatalf("构造会话存储失败: %v", err)
	}
	sess, err := session.New("tester", "t", "/tmp/proj", store, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	out := pb.Build("sid", sess)
	if strings.Contains(out, "## 行为规则") {
		t.Errorf("SOUL 段不应注入外部标题")
	}
	if !strings.Contains(out, "## 核心逻辑\n\n由 SOUL.md 自带的标题") {
		t.Errorf("SOUL.md 正文应原样拼接（标题由文件自带），实际输出:\n%s", out)
	}
}
