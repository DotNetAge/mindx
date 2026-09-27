package skillstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/DotNetAge/goharness/skill"
)

// skillDirName 是 Agent 目录下独有技能库的固定子目录名。
const skillDirName = "skills"

// Registry 是 mindx 侧的技能注册表：实现 goharness skill.SkillRegistry 检索 SPI
// （Skill 工具按名称加载指令），并扩展 List 供管理界面（skill.list RPC）列举。
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*skill.Skill
}

// NewRegistry 创建空的技能注册表。
func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]*skill.Skill)}
}

// 编译期接口检查：goharness 只消费检索 SPI。
var _ skill.SkillRegistry = (*Registry)(nil)

// RegisterSkill 注册技能，同名技能后注册者覆盖先注册者。
func (r *Registry) RegisterSkill(sk *skill.Skill) error {
	if sk == nil || strings.TrimSpace(sk.Name) == "" {
		return fmt.Errorf("技能名称不能为空")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[sk.Name] = sk
	return nil
}

// GetSkill 按名称检索技能；未找到时返回 goharness 的 ErrSkillNotFound。
func (r *Registry) GetSkill(name string) (*skill.Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sk, ok := r.skills[name]
	if !ok {
		return nil, skill.ErrSkillNotFound
	}
	return sk, nil
}

// List 返回全部技能，按名称排序保证输出稳定。
func (r *Registry) List() []*skill.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	list := make([]*skill.Skill, 0, len(names))
	for _, name := range names {
		list = append(list, r.skills[name])
	}
	return list
}

// Store 管理技能三级库的装载与运行时注册表组装（PR-PROMPTS 第三节）。
//
//   - 全局库：~/.mindx/skills，全部 Agent 共享的"最佳实践"库；
//   - Agent 级库：agents/<name>/skills，Agent 独有技能；
//   - 项目级库：<ProjectDir>/.agents/skills，发现式的"可能"经验（军规：
//     动态技能绝不进入系统提示词——经 mindx skills discovery 发现、
//     Skill 工具按需加载回退解析，见 promote.go 的 ResolveProject/晋升管线）；
//   - 同名覆盖：运行时注册顺序为"先全局、后 Agent 级"，同名技能
//     Agent 级版本生效（逻辑重写），分发包安装与用户手动放置均走此语义。
type Store struct {
	globalDir string
	agentsDir string

	mu     sync.RWMutex
	global *Registry
}

// NewStore 创建技能存储并加载全局库。
// 全局库加载产生的单技能错误会汇总返回（不阻断创建，成功加载的技能照常可用）；
// 目录不存在视为空库，不视为错误。
func NewStore(globalDir, agentsDir string) (*Store, error) {
	if strings.TrimSpace(globalDir) == "" {
		return nil, fmt.Errorf("全局技能目录不能为空")
	}
	s := &Store{
		globalDir: globalDir,
		agentsDir: agentsDir,
	}
	err := s.ReloadGlobal()
	return s, err
}

// Global 返回全局库注册表（只读访问，管理界面与 RPC 消费）。
func (s *Store) Global() *Registry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.global
}

// GlobalDir 返回全局技能库目录。
func (s *Store) GlobalDir() string { return s.globalDir }

// AgentSkillDir 返回指定 Agent 的独有技能库目录（agents/<name>/skills）。
// agentName 为空时返回空字符串。
func (s *Store) AgentSkillDir(agentName string) string {
	if strings.TrimSpace(agentName) == "" || s.agentsDir == "" {
		return ""
	}
	return filepath.Join(s.agentsDir, agentName, skillDirName)
}

// ReloadGlobal 重扫全局技能库并原子替换内存注册表。
// 单技能加载失败跳过并汇入返回错误，不阻断其余技能装载。
func (s *Store) ReloadGlobal() error {
	reg, errs := s.loadDir(s.globalDir)
	s.mu.Lock()
	s.global = reg
	s.mu.Unlock()

	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Errorf("全局技能库部分技能加载失败：%s", strings.Join(msgs, "；"))
}

// MergeGlobalDir 将另一个平铺技能库（<skill-name>/SKILL.md 结构）并入当前全局
// 注册表，同名技能后注册者覆盖。用于市场打包等需要跨多库解析技能引用的场景。
// 库内单技能加载失败跳过并汇入返回错误，不阻断其余技能并入。
func (s *Store) MergeGlobalDir(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("待并入的技能库目录不能为空")
	}
	s.mu.RLock()
	reg := s.global
	s.mu.RUnlock()

	_, errs := s.loadInto(reg, dir)
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Errorf("技能库 %s 部分技能并入失败：%s", dir, strings.Join(msgs, "；"))
}

// RegistryFor 组装指定 Agent 的运行时技能注册表：全局库 + Agent 级覆盖。
// 同名技能以 Agent 级版本为准（后注册覆盖）。agentName 为空或 Agent 无独有
// 技能目录时，等价于全局库副本。
func (s *Store) RegistryFor(agentName string) *Registry {
	// 复制全局库，避免运行时注册表与管理视图共享同一份 map 导致互相干扰。
	reg := NewRegistry()
	s.mu.RLock()
	for _, sk := range s.global.List() {
		_ = reg.RegisterSkill(sk)
	}
	s.mu.RUnlock()

	if dir := s.AgentSkillDir(agentName); dir != "" {
		if _, errs := s.loadInto(reg, dir); len(errs) > 0 {
			for _, err := range errs {
				fmt.Fprintf(os.Stderr, "[技能加载器] 警告：%v\n", err)
			}
		}
	}
	return reg
}

// AgentSkills 独立装载指定 Agent 的私有技能库（不含全局库），
// 供管理界面展示 Agent 级技能（晋升"提取到全局"操作的来源视图）。
func (s *Store) AgentSkills(agentName string) *Registry {
	reg, _ := s.loadDir(s.AgentSkillDir(agentName))
	return reg
}

// loadDir 扫描一个技能库目录，返回装载结果与错误汇总。
func (s *Store) loadDir(dir string) (*Registry, []error) {
	reg := NewRegistry()
	_, errs := s.loadInto(reg, dir)
	return reg, errs
}

// dirSkillNames 列出一个技能库目录下的技能目录名（即 Agent 级技能名集合）。
// 目录不存在时返回 nil。
func (s *Store) dirSkillNames(dir string) map[string]bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names[entry.Name()] = true
		}
	}
	return names
}

// Catalog 组装指定 Agent 的技能摘要目录（系统提示词"技能目录"段的数据源）。
//
// 选取语义与旧版一致：
//   - Agent 级技能库（agents/<name>/skills）中的技能自动入选；
//   - 全局库中属于 Agent 声明（declared，即 frontmatter skills 引用）的技能入选；
//   - 同名技能以 Agent 级版本为准（RegistryFor 的覆盖顺序）。
//
// 返回列表按名称排序，保证提示词前缀稳定（KV 缓存友好）。
func (s *Store) Catalog(agentName string, declared []string) []*skill.Skill {
	reg := s.RegistryFor(agentName)

	agentSkills := s.dirSkillNames(s.AgentSkillDir(agentName))
	declaredSet := make(map[string]bool, len(declared))
	for _, name := range declared {
		declaredSet[name] = true
	}

	var catalog []*skill.Skill
	for _, sk := range reg.List() {
		if agentSkills[sk.Name] || declaredSet[sk.Name] {
			catalog = append(catalog, sk)
		}
	}
	return catalog
}

// loadInto 将一个技能库目录中的全部技能注册进 reg。
// 目录不存在视为空库（不报错）；每个技能独立装载，SKILL.md 解析错误跳过并收集；
// 依赖未满足的技能照常注册，但将告警汇入返回的 errs（供管理界面提示用户）。
func (s *Store) loadInto(reg *Registry, dir string) (*Registry, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return reg, []error{fmt.Errorf("读取技能目录 %s 失败：%w", dir, err)}
	}

	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(dir, entry.Name())
		sk, warnings, err := LoadSkillFromDir(skillDir, "filesystem")
		if err != nil {
			// SKILL.md 解析错误（缺必填字段、YAML 错误）等——硬错误，跳过
			errs = append(errs, fmt.Errorf("跳过 %s：%w", skillDir, err))
			continue
		}
		if sk == nil {
			continue
		}
		// 依赖未满足的告警——技能仍然注册，但记下 warnings 供展示
		for _, w := range warnings {
			errs = append(errs, fmt.Errorf("%s：依赖告警 — %s", entry.Name(), w))
		}
		if err := reg.RegisterSkill(sk); err != nil {
			errs = append(errs, err)
		}
	}
	return reg, errs
}
