// Package bundle 实现 Agent / Skill 分发包的导出、安装与 COS 静态市场客户端。
//
// 分发包格式（PR-PROMPTS 第四节定稿；Skill 包是 Agent 包的子集）：
//
//	<name>.mindpkg（zip 容器）
//	├── manifest.json                    包清单：{ format, kind, name, description, icon, skills }
//	├── agent/                           kind=agent 时存在：Agent 目录内容
//	│   ├── IDENTITY.md
//	│   ├── SOUL.md
//	│   └── skills/<skill-name>/...     Agent 级技能（含同名覆盖版本）+ 引用的全局技能副本
//	└── skills/<skill-name>/...          kind=skill 时存在：单个技能目录
//
// 安装展开规则（分发格式 ≠ 存储格式，冗余只存在于传输层）：
//   - Agent 包：agent/ → agents/<name>/；包内技能与全局库同名 → 落 Agent 目录
//     skills/（Agent 级覆盖 = 逻辑重写），不同名 → 进全局库；
//   - Skill 包：skills/<name>/ → 全局库；
//   - 同名冲突默认拒绝，覆盖需显式确认（Overwrite）。
package bundle

import "fmt"

// PackageFormat 是包内清单的格式版本（当前唯一版本）。
const PackageFormat = 1

// PackageKind 标识分发包类型。
type PackageKind string

const (
	// KindAgent Agent 分发包（agent 目录 + 实际生效的技能副本）。
	KindAgent PackageKind = "agent"
	// KindSkill Skill 分发包（单技能，Agent 包的子集）。
	KindSkill PackageKind = "skill"
)

// zip 容器内的固定路径前缀。
const (
	zipPathManifest = "manifest.json"
	zipPrefixAgent  = "agent/"
	zipPrefixSkills = "skills/"

	// agent 目录内固定文件名（与 agentstore 的目录格式一致）。
	identityFileName = "IDENTITY.md"
	soulFileName     = "SOUL.md"
)

// Manifest 是分发包容器内的清单（manifest.json）。
type Manifest struct {
	Format      int         `json:"format"`
	Kind        PackageKind `json:"kind"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Icon        string      `json:"icon,omitempty"`
	// Role 岗位头衔（kind=agent，来自 IDENTITY frontmatter；市场卡片大标题用）。
	Role string `json:"role,omitempty"`
	// NickName 昵称（kind=agent，2-3 字外号，展示主名；统一显示规则 = 昵称 + Role 小字）。
	NickName string `json:"nick_name,omitempty"`
	// Category 业务分类（kind=agent，中文，取自 IDENTITY.md frontmatter category；
	// 市场陈列与本地注册表同口径，三端统一用它过滤与展示）。
	Category string `json:"category,omitempty"`
	// Skills 记录包内包含的技能名（kind=agent）；安装器据此报告落位结果。
	Skills []string `json:"skills,omitempty"`
	// SkillNames 技能中文展示名映射（技能名 → SKILL.md metadata.name_zh；
	// Agent 包记录包内全部技能，Skill 包记录其自身；未声明中文名的不收录，
	// 展示端回退原名）。市场卡片展示用。
	SkillNames map[string]string `json:"skill_names,omitempty"`
	// SkillDescs 技能描述映射（技能名 → SKILL.md frontmatter description）；
	// 收录语义同 SkillNames。市场详情页展示用（未安装包的技能描述来源）。
	SkillDescs map[string]string `json:"skill_descs,omitempty"`
	// Dependencies 技能的外部依赖声明（分发包格式契约字段，从外部 manifest.json
	// 透传保留；当前打包端不写入、安装端不执行，仅供生态元数据不丢失）。
	Dependencies *SkillDependencies `json:"dependencies,omitempty"`
}

// SkillDependencies 是一个 skill 的全部外部依赖（三层模型）。
type SkillDependencies struct {
	// Runtime 生态级运行时要求（跨 skill 共享）：{"node": ">=18", "python": ">=3.9"}。
	Runtime map[string]string `json:"runtime,omitempty"`
	// Bins 需要在 PATH 中找到的二进制名列表（如 lark-cli、ffmpeg）。
	Bins []string `json:"bins,omitempty"`
	// Env 需要设置的环境变量名列表。
	Env []string `json:"env,omitempty"`
	// Install 自定义安装脚本（相对于 skill 目录）。优先于默认 npm/pip install。
	Install string `json:"install,omitempty"`
}

// validate 校验清单完整性。
func (m *Manifest) validate() error {
	if m == nil {
		return fmt.Errorf("分发包缺少 manifest.json")
	}
	if m.Format != PackageFormat {
		return fmt.Errorf("不支持的包格式版本：%d（当前支持 %d）", m.Format, PackageFormat)
	}
	if m.Kind != KindAgent && string(m.Kind) != "skill" {
		return fmt.Errorf("未知的分发包类型：%q", m.Kind)
	}
	if m.Name == "" {
		return fmt.Errorf("包清单缺少 name 字段")
	}
	return nil
}

// InstallResult 描述一次安装的落位结果（RPC 返回给前端展示）。
type InstallResult struct {
	Kind PackageKind `json:"kind"`
	Name string      `json:"name"`
	// SkillsGlobal 落入全局库的技能名。
	SkillsGlobal []string `json:"skills_global,omitempty"`
	// SkillsAgent 落入 Agent 级库的技能名（同名覆盖，逻辑重写）。
	SkillsAgent []string `json:"skills_agent,omitempty"`
	// Overwritten 是否覆盖了已存在的同名目标（用户显式确认）。
	Overwritten bool `json:"overwritten,omitempty"`
}
