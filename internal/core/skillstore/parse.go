// Package skillstore 提供 mindx 侧的技能三级库装载与解析。
//
// 职责（PR-PROMPTS 第二、三节）：
//   - SKILL.md 的目录扫描与 frontmatter 解析（自 goharness/skill 迁移）；
//   - 运行时注册表组装：全局库（~/.mindx/skills）+ Agent 级库
//     （agents/<name>/skills），同名技能"Agent 级覆盖全局级"（逻辑重写）；
//   - goharness 只消费检索 SPI（skill.SkillRegistry），本包是注入方。
//
// 项目级库（<ProjectDir>/.skills）的发现式载入属 P2 晋升管线，不在本包。
package skillstore

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/goharness/skill"
	"gopkg.in/yaml.v3"
)

// ValidateSkillName 校验技能名称（agentskills.io 规范）。
func ValidateSkillName(name string) error {
	if len(name) < 1 || len(name) > 64 {
		return fmt.Errorf("技能名称长度必须为 1-64 个字符，实际为 %d", len(name))
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return fmt.Errorf("技能名称不能以连字符开头或结尾：%q", name)
	}
	if strings.Contains(name, "--") {
		return fmt.Errorf("技能名称不能包含连续的连字符：%q", name)
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return fmt.Errorf("技能名称只能包含小写字母、数字和连字符：%q", name)
		}
	}
	return nil
}

// ValidateSkillDescription 校验技能描述长度。
func ValidateSkillDescription(desc string) error {
	if len(desc) < 1 || len(desc) > 1024 {
		return fmt.Errorf("技能描述长度必须为 1-1024 个字符，实际为 %d", len(desc))
	}
	return nil
}

// AllowedToolsList 兼容 YAML 中字符串或字符串列表两种写法，反序列化后以空格连接。
type AllowedToolsList string

// UnmarshalYAML 实现 yaml.Unmarshaler。
func (a *AllowedToolsList) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		*a = AllowedToolsList(s)
		return nil
	}
	var list []string
	if err := unmarshal(&list); err == nil {
		*a = AllowedToolsList(strings.Join(list, " "))
		return nil
	}
	return fmt.Errorf("allowed-tools 必须是字符串或字符串列表")
}

// skillFrontmatter 是 SKILL.md frontmatter 的解析载体。
// license / compatibility / metadata 仅用于依赖检查（metadata.requires），
// 其余键解析后丢弃（存储字段归应用侧，goharness Skill 结构已瘦身）。
type skillFrontmatter struct {
	Name         string           `yaml:"name"`
	Description  string           `yaml:"description"`
	Metadata     map[string]any   `yaml:"metadata"`
	AllowedTools AllowedToolsList `yaml:"allowed-tools"`
}

// Requires 声明技能的运行时依赖。
type Requires struct {
	// Bins 需要在 PATH 中找到的二进制名列表（如 lark-cli、ffmpeg）。
	Bins []string `yaml:"bins"`
	// Env 需要设置的环境变量名列表。
	Env []string `yaml:"env"`
	// Runtime 生态级运行时版本要求（跨 skill 共享），如 node: ">=18"、python: ">=3.9"。
	// key 是运行时名（node / python / dotnet / go / rust / java 等），
	// value 是 semver 范围字符串（如 ">=18"、"^3.9"、"8.x"）。
	Runtime map[string]string `yaml:"runtime"`
	// Install 自定义安装脚本路径（相对于 skill 目录），优先于默认的 npm/pip install。
	Install string `yaml:"install"`
}

// SkillDetail 在运行时 Skill 之上补充应用侧展示字段：
// requires 依赖声明与 metadata 原始键值，仅 CLI / 管理面使用，
// 不进入 goharness 运行时 SPI（Skill 结构保持瘦身）。
type SkillDetail struct {
	*skill.Skill
	Requires *Requires      // metadata.requires 依赖声明（可为 nil）
	Metadata map[string]any // frontmatter metadata 原始键值（可为 nil）
}

// LoadSkillDetailFromDir 加载 Skill 及其展示字段。
// 目录不含 SKILL.md 时返回 (nil, nil)；依赖未满足时返回错误，由调用方跳过。
func LoadSkillDetailFromDir(dir, source string) (*SkillDetail, error) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 SKILL.md 失败：%w", err)
	}

	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, err
	}
	if fm.Name == "" {
		return nil, fmt.Errorf("SKILL.md 缺少前置元数据中必需的 'name' 字段")
	}
	if fm.Description == "" {
		return nil, fmt.Errorf("SKILL.md 缺少前置元数据中必需的 'description' 字段")
	}

	name := fm.Name
	if err := ValidateSkillName(name); err != nil {
		// 名称不规范时尝试清洗（空格转连字符、转小写）
		sanitized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
		if err2 := ValidateSkillName(sanitized); err2 != nil {
			return nil, err
		}
		name = sanitized
	}
	if err := ValidateSkillDescription(fm.Description); err != nil {
		return nil, err
	}

	// 指令模板变量替换
	instructions := strings.TrimSpace(body)
	instructions = strings.ReplaceAll(instructions, "{base_dir}", dir)
	instructions = strings.ReplaceAll(instructions, "{skill_name}", name)

	req := extractRequires(fm.Metadata)
	if err := verifyDependencies(req); err != nil {
		return nil, fmt.Errorf("技能 %q 的依赖检查失败：%w", name, err)
	}

	return &SkillDetail{
		Skill: &skill.Skill{
			Name:         name,
			Description:  fm.Description,
			AllowedTools: string(fm.AllowedTools),
			Instructions: instructions,
			RootDir:      dir,
			Source:       source,
		},
		Requires: req,
		Metadata: fm.Metadata,
	}, nil
}

// LoadSkillFromDir 从单个技能目录加载运行时 Skill（不含展示字段）。
// 目录不含 SKILL.md 时返回 (nil, nil)；依赖未满足时返回错误，由调用方跳过。
func LoadSkillFromDir(dir, source string) (*skill.Skill, error) {
	d, err := LoadSkillDetailFromDir(dir, source)
	if err != nil || d == nil {
		return nil, err
	}
	return d.Skill, nil
}

// LoadSkillDisplayName 读取技能目录 SKILL.md 的中文展示名（metadata.name_zh）。
// 仅用于导出打包等展示场景：不校验名称规范与运行时依赖；目录缺失或解析失败
// 一律返回空串，由调用方回退为技能原名（展示字段的兜底语义，不构成错误）。
func LoadSkillDisplayName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return ""
	}
	fm, _, err := parseFrontmatter(string(data))
	if err != nil {
		return ""
	}
	zh, _ := fm.Metadata["name_zh"].(string)
	return strings.TrimSpace(zh)
}

// LoadSkillDescription 读取技能目录 SKILL.md 的 frontmatter description。
// 仅用于导出打包等展示场景：不校验名称规范与运行时依赖；目录缺失或解析失败
// 一律返回空串，由调用方回退缺省展示（不构成错误）。
func LoadSkillDescription(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return ""
	}
	fm, _, err := parseFrontmatter(string(data))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(fm.Description)
}

// LoadSkillMetadata 读取技能目录 SKILL.md 的 frontmatter metadata 原始键值
// （name_zh / version 等展示字段）。仅用于 RPC 展示场景：不校验名称规范与
// 运行时依赖；目录缺失或解析失败一律返回 nil，由调用方回退缺省展示（不构成错误）。
func LoadSkillMetadata(dir string) map[string]any {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil
	}
	fm, _, err := parseFrontmatter(string(data))
	if err != nil {
		return nil
	}
	return fm.Metadata
}

// parseFrontmatter 分离 frontmatter 与正文并解析 YAML。
func parseFrontmatter(content string) (skillFrontmatter, string, error) {
	var fm skillFrontmatter

	content = strings.TrimLeft(content, "\n\r")
	if !strings.HasPrefix(content, "---") {
		return fm, content, fmt.Errorf("SKILL.md 必须以 YAML 前置元数据（---）开头")
	}
	rest := content[len("---"):]
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return fm, content, fmt.Errorf("SKILL.md 的 YAML 前置元数据未闭合（缺少结尾的 ---）")
	}
	yamlBlock := rest[:closeIdx]
	body := rest[closeIdx+len("---")+1:]

	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return fm, body, fmt.Errorf("解析 YAML 前置元数据失败：%w", err)
	}
	return fm, body, nil
}

// extractRequires 从 metadata 中提取 requires 声明（规范：扩展键放在 metadata）。
func extractRequires(metadata map[string]any) *Requires {
	raw, ok := metadata["requires"]
	if !ok {
		return nil
	}
	reqMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	r := &Requires{}
	if bins, ok := reqMap["bins"].([]any); ok {
		for _, b := range bins {
			if s, ok := b.(string); ok {
				r.Bins = append(r.Bins, s)
			}
		}
	}
	if envs, ok := reqMap["env"].([]any); ok {
		for _, e := range envs {
			if s, ok := e.(string); ok {
				r.Env = append(r.Env, s)
			}
		}
	}
	if rt, ok := reqMap["runtime"].(map[string]any); ok {
		r.Runtime = make(map[string]string, len(rt))
		for k, v := range rt {
			if s, ok := v.(string); ok {
				r.Runtime[k] = strings.TrimSpace(s)
			}
		}
	}
	if inst, ok := reqMap["install"].(string); ok {
		r.Install = strings.TrimSpace(inst)
	}
	if len(r.Bins) == 0 && len(r.Env) == 0 && len(r.Runtime) == 0 && r.Install == "" {
		return nil
	}
	return r
}

// verifyDependencies 检查声明的运行时依赖是否满足。
func verifyDependencies(r *Requires) error {
	if r == nil {
		return nil
	}
	for _, bin := range r.Bins {
		bin = strings.TrimSpace(bin)
		if bin == "" {
			continue
		}
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("在 PATH 中未找到必需的二进制文件 %q", bin)
		}
	}
	for _, key := range r.Env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if os.Getenv(key) == "" {
			return fmt.Errorf("必需的环境变量 %q 未设置", key)
		}
	}
	for runtime, constraint := range r.Runtime {
		if err := checkRuntimeVersion(runtime, constraint); err != nil {
			return err
		}
	}
	return nil
}

// checkRuntimeVersion 检查运行时版本是否满足约束（简化版，支持 ">=N"、">N"、"N.x"、"N"）。
func checkRuntimeVersion(runtime, constraint string) error {
	bin := runtime
	switch runtime {
	case "python":
		bin = "python3"
	case "node":
		bin = "node"
	case "dotnet":
		bin = "dotnet"
	case "go":
		bin = "go"
	case "rust":
		bin = "rustc"
	case "java":
		bin = "java"
	}

	// 先检查二进制是否存在
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("需要运行时 %q（约束 %q），但在 PATH 中未找到 %q。请先安装 %s", runtime, constraint, bin, runtime)
	}

	// 读版本号
	ver, err := readRuntimeVersion(bin)
	if err != nil {
		return fmt.Errorf("读取 %q 版本失败：%w", runtime, err)
	}
	if ver == "" {
		return fmt.Errorf("无法解析 %q 的版本号（原始输出为空）", runtime)
	}

	// 简单比较：>=N / >N / N
	major, minor := parseSimpleVersion(ver)
	if major == 0 {
		return fmt.Errorf("无法解析 %q 的版本号：%s", runtime, ver)
	}

	if !satisfiesSimpleConstraint(major, minor, constraint) {
		return fmt.Errorf("%q 版本 %s 不满足约束 %q，请升级或更换版本", runtime, ver, constraint)
	}
	return nil
}

// readRuntimeVersion 执行 <bin> --version 并返回版本号字符串。
func readRuntimeVersion(bin string) (string, error) {
	var args []string
	switch bin {
	case "python3":
		args = []string{"--version"} // stderr 输出
	case "dotnet":
		args = []string{"--version"}
	case "go":
		args = []string{"version"}
	case "java":
		args = []string{"-version"} // stderr 输出
	default:
		args = []string{"--version"}
	}

	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		// 有些工具即使返回非零但版本信息已输出
		if len(out) == 0 {
			return "", fmt.Errorf("执行 %q 失败：%v", bin, err)
		}
	}
	return string(out), nil
}

// parseSimpleVersion 从字符串中提取主版本号和次版本号（第一个数字段）。
func parseSimpleVersion(s string) (int, int) {
	// 在输出中找形如 x.y.z 的版本号
	idx := strings.IndexAny(s, "0123456789")
	if idx < 0 {
		return 0, 0
	}
	rest := s[idx:]

	// 提取主版本号
	major := 0
	for _, c := range rest {
		if c >= '0' && c <= '9' {
			major = major*10 + int(c-'0')
		} else {
			break
		}
	}
	if major == 0 {
		return 0, 0
	}

	// 找次版本号（点号之后）
	minor := 0
	dotIdx := strings.Index(rest, ".")
	if dotIdx >= 0 {
		sub := rest[dotIdx+1:]
		for _, c := range sub {
			if c >= '0' && c <= '9' {
				minor = minor*10 + int(c-'0')
			} else {
				break
			}
		}
	}
	return major, minor
}

// satisfiesSimpleConstraint 判断主/次版本是否满足简单约束（>=N、>N、N.x、N）。
func satisfiesSimpleConstraint(major, minor int, constraint string) bool {
	c := strings.TrimSpace(constraint)
	switch {
	case strings.HasPrefix(c, ">="):
		target, _ := parseVersionStr(strings.TrimPrefix(c, ">="))
		return major > target || (major == target)
	case strings.HasPrefix(c, ">"):
		target, _ := parseVersionStr(strings.TrimPrefix(c, ">"))
		return major > target
	case strings.HasPrefix(c, "^"):
		// ^N → >=N, <(N+1).0.0
		target, _ := parseVersionStr(strings.TrimPrefix(c, "^"))
		return major >= target && major < target+1
	case strings.HasSuffix(c, ".x"):
		target, _ := parseVersionStr(strings.TrimSuffix(c, ".x"))
		return major == target
	default:
		// 纯数字或数字范围：默认 >=
		target, _ := parseVersionStr(c)
		return major >= target
	}
}

func parseVersionStr(s string) (int, int) {
	s = strings.TrimSpace(s)
	dot := strings.Index(s, ".")
	if dot < 0 {
		return parseIntFirst(s), 0
	}
	major := parseIntFirst(s[:dot])
	return major, parseIntFirst(s[dot+1:])
}

func parseIntFirst(s string) int {
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
