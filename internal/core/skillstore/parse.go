// Package skillstore 提供 mindx 侧的技能三级库装载与解析。
//
// 职责（PR-PROMPTS 第二、三节）：
//   - SKILL.md 的目录扫描与 frontmatter 解析（自 goharness/skill 迁移）；
//   - 运行时注册表组装：全局库（~/.mindx/skills）+ Agent 级库
//     （agents/<name>/skills），同名技能"Agent 级覆盖全局级"（逻辑重写）；
//   - goharness 只消费检索 SPI（skill.SkillRegistry），本包是注入方。
//
// 项目级库（<ProjectDir>/.agents/skills）的发现式载入属 P2 晋升管线，不在本包。
package skillstore

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

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
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
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

// OAuthRequirement 声明技能所需的 OAuth 授权（requires.oauth）。
// provider 引用 client-provider.yml 注册表名；env 是授权成功后令牌落库的
// 凭据名，存储键为 <skill_name>:<env>（三层解析链第一层）。
type OAuthRequirement struct {
	Provider string `yaml:"provider"`
	Env      string `yaml:"env"`
}

// SkillEnvRequirement 声明技能所需单个环境变量的收集信息
// （requires.envs 对象形态条目 {name, prompt, url, sensitive}）。
// name 缺失时整条忽略（对齐 oauth「字段不齐整条忽略并告警」纪律）；
// 字符串形态条目只声明变量名，首用收集时无法给出说明文案。
type SkillEnvRequirement struct {
	// Name 环境变量名（如 LARK_APP_SECRET）。
	Name string
	// Prompt 首用收集时展示给用户的说明文案（去哪申请、贴什么值）。
	Prompt string
	// URL 凭据申请入口链接（可为空）。
	URL string
	// Sensitive 掩码输入标记：true 表示敏感凭据，收集界面掩码显示、
	// 落库前不得出现在日志或通知中。
	Sensitive bool
}

// Requires 声明技能的运行时依赖。
type Requires struct {
	// Bins 需要在 PATH 中找到的二进制名列表（如 lark-cli、ffmpeg）。
	Bins []string `yaml:"bins"`
	// Env 环境变量名名单。定稿键为 envs，兼容存量 env 写法（两者都收）；
	// 字符串条目与对象条目的 name 都并入此名单（MissingEnvs / verifyDependencies 出口兼容）。
	Env []string `yaml:"-"`
	// Envs 对象形态条目（{name, prompt, url, sensitive}），与 Env 并存：
	// 字符串条目只进 Env；对象条目进 Envs 且 name 同时并入 Env。
	// 首用收集交互需要 prompt/url/sensitive 字段时读此列表。
	Envs []SkillEnvRequirement `yaml:"-"`
	// Runtime 生态级运行时版本要求（跨 skill 共享），如 node: ">=18"、python: ">=3.9"。
	// key 是运行时名（node / python / dotnet / go / rust / java 等），
	// value 是 semver 范围字符串（如 ">=18"、"^3.9"、"8.x"）。
	Runtime map[string]string `yaml:"runtime"`
	// OAuth 技能运行所需 OAuth 授权（provider + env，缺失触发准备流程）。
	OAuth *OAuthRequirement `yaml:"oauth"`
}

// SkillDetail 在运行时 Skill 之上补充应用侧展示字段：
// requires 依赖声明与 metadata 原始键值，仅 CLI / 管理面使用，
// 不进入 goharness 运行时 SPI（Skill 结构保持瘦身）。
type SkillDetail struct {
	*skill.Skill
	Requires *Requires      // metadata.requires 依赖声明（可为 nil）
	Metadata map[string]any // frontmatter metadata 原始键值（可为 nil）
	// InstallCommands metadata.install 修复动作命令块（平台名 → 命令列表，
	// 如 darwin / linux；定稿：安装时展示命令清单，用户确认一次后自动执行）。
	// 与 requires 平级（metadata.install），可为 nil。
	InstallCommands map[string][]string
}

// LoadSkillDetailFromDir 加载 Skill 及其展示字段。
// 目录不含 SKILL.md 时返回 (nil, nil)；SKILL.md 解析错误（缺必填字段、YAML 错误）
// 返回硬错误；依赖未满足不阻断注册，技能照常装载并通过 warnings 告知调用方。
//
// warnings 返回值：收集所有依赖未满足的告警（二进制缺失、版本不满足等）；
// 正常情况下返回 nil。调用方可选择记录到日志或展示给用户。
func LoadSkillDetailFromDir(dir, source string) (*SkillDetail, []string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("读取 SKILL.md 失败：%w", err)
	}

	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, nil, err
	}
	if fm.Name == "" {
		return nil, nil, fmt.Errorf("前置元数据缺少必需的 'name' 字段（SKILL.md 头部）")
	}
	if fm.Description == "" {
		return nil, nil, fmt.Errorf("前置元数据缺少必需的 'description' 字段（SKILL.md 头部）")
	}

	name := fm.Name
	if err := ValidateSkillName(name); err != nil {
		// 名称不规范时尝试清洗（空格转连字符、转小写）
		sanitized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
		if err2 := ValidateSkillName(sanitized); err2 != nil {
			return nil, nil, err
		}
		name = sanitized
	}
	if err := ValidateSkillDescription(fm.Description); err != nil {
		return nil, nil, err
	}

	// 指令模板变量替换
	instructions := strings.TrimSpace(body)
	instructions = strings.ReplaceAll(instructions, "{base_dir}", dir)
	instructions = strings.ReplaceAll(instructions, "{skill_name}", name)

	req := extractRequires(fm.Metadata)
	warnings := verifyDependencies(req)

	return &SkillDetail{
		Skill: &skill.Skill{
			Name:         name,
			Description:  fm.Description,
			AllowedTools: string(fm.AllowedTools),
			Instructions: instructions,
			RootDir:      dir,
			Source:       source,
		},
		Requires:        req,
		Metadata:        fm.Metadata,
		InstallCommands: extractInstallCommands(fm.Metadata),
	}, warnings, nil
}

// LoadSkillFromDir 从单个技能目录加载运行时 Skill（不含展示字段）。
// 目录不含 SKILL.md 时返回 (nil, nil)；SKILL.md 解析错误返回硬错误；
// 依赖未满足不阻断注册，warnings 收集告警信息供调用方参考。
func LoadSkillFromDir(dir, source string) (*skill.Skill, []string, error) {
	d, warnings, err := LoadSkillDetailFromDir(dir, source)
	if err != nil || d == nil {
		return nil, warnings, err
	}
	return d.Skill, warnings, nil
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
		return fm, content, fmt.Errorf("文件必须以 YAML 前置元数据（---）开头")
	}
	rest := content[len("---"):]
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return fm, content, fmt.Errorf("YAML 前置元数据未闭合（缺少结尾的 ---）")
	}
	yamlBlock := rest[:closeIdx]
	body := rest[closeIdx+len("---")+1:]

	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return fm, body, fmt.Errorf("解析 YAML 前置元数据失败：%w", err)
	}
	return fm, body, nil
}

// extractRequires 从 metadata 中提取 requires 声明（规范：扩展键放在 metadata）。
// env 名单定稿键为 envs，兼容存量 env 写法；oauth 为 provider+env 两字段对象。
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
	// env 名单：envs（定稿）优先，env（存量兼容）兜底；条目支持字符串或
	// 对象 {name, prompt, url, sensitive} 两种形态——对象 name 缺失整条忽略
	//（告警由 CheckQuality 的 quality-envs 检查兜底，不阻断装载）。
	envsRaw, ok := reqMap["envs"].([]any)
	if !ok {
		envsRaw, ok = reqMap["env"].([]any)
	}
	if ok {
		for _, item := range envsRaw {
			switch v := item.(type) {
			case string:
				r.Env = append(r.Env, v)
			case map[string]any:
				e := parseSkillEnvRequirement(v)
				if e == nil {
					continue
				}
				r.Envs = append(r.Envs, *e)
				r.Env = append(r.Env, e.Name)
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
	if oauthMap, ok := reqMap["oauth"].(map[string]any); ok {
		oauth := &OAuthRequirement{}
		if s, ok := oauthMap["provider"].(string); ok {
			oauth.Provider = strings.TrimSpace(s)
		}
		if s, ok := oauthMap["env"].(string); ok {
			oauth.Env = strings.TrimSpace(s)
		}
		if oauth.Provider != "" && oauth.Env != "" {
			r.OAuth = oauth
		}
	}
	if len(r.Bins) == 0 && len(r.Env) == 0 && len(r.Runtime) == 0 && r.OAuth == nil {
		return nil
	}
	return r
}

// stringList 把 YAML 反序列化出的 any 列表收窄为字符串列表。
func stringList(raw []any) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// parseSkillEnvRequirement 把 YAML 反序列化出的对象条目收窄为 SkillEnvRequirement。
// name 缺失（空/非字符串/空白）时返回 nil，调用方整条忽略。
func parseSkillEnvRequirement(raw map[string]any) *SkillEnvRequirement {
	name := strings.TrimSpace(stringField(raw, "name"))
	if name == "" {
		return nil
	}
	e := &SkillEnvRequirement{
		Name:   name,
		Prompt: strings.TrimSpace(stringField(raw, "prompt")),
		URL:    strings.TrimSpace(stringField(raw, "url")),
	}
	if b, ok := raw["sensitive"].(bool); ok {
		e.Sensitive = b
	}
	return e
}

// stringField 从 YAML 反序列化出的 map 中取字符串字段（类型不符返回空串）。
func stringField(raw map[string]any, key string) string {
	if v, ok := raw[key].(string); ok {
		return v
	}
	return ""
}

// extractInstallCommands 从 metadata 中提取 install 命令块（与 requires 平级）：
// 平台名（darwin / linux 等）→ 命令列表。仅收字符串数组值，空块返回 nil。
func extractInstallCommands(metadata map[string]any) map[string][]string {
	raw, ok := metadata["install"]
	if !ok {
		return nil
	}
	installMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string][]string, len(installMap))
	for platform, cmds := range installMap {
		list, ok := cmds.([]any)
		if !ok {
			continue
		}
		commands := stringList(list)
		if len(commands) > 0 {
			out[platform] = commands
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MissingBins 返回声明但未在执行端找到的二进制名（准备流程 needs_confirm 判定
// 与「环境不满足」报错的输入）。三层查找链同 verifyDependencies（PATH → shell
// → 常见目录扫描）。
func MissingBins(r *Requires) []string {
	if r == nil {
		return nil
	}
	var missing []string
	for _, bin := range r.Bins {
		bin = strings.TrimSpace(bin)
		if bin == "" {
			continue
		}
		if _, err := resolveBin(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	return missing
}

// 依赖探测结果缓存（进程级）：resolveBin / checkRuntimeVersion 的结果按名称缓存。
// GUI 进程 PATH 阉割时二层探测会起 zsh 登录 shell（1-3 秒/次），全库重扫
// （安装/装载技能）对同类依赖反复探测曾把一次安装拖到 20+ 秒；缓存后同类
// 依赖一个进程只探一次。探测结果是环境属性，进程生命周期内不变，无失效问题。
var depCache sync.Map // key: string, value: cachedDepResult

type cachedDepResult struct {
	path  string
	verr  error // resolveBin 的错误
	rtErr error // checkRuntimeVersion 的错误
}

// verifyDependencies 检查声明的运行时依赖是否满足。
// 返回值为告警列表：依赖缺失 / 版本不满足时收集详细告警，不阻断技能注册。
// r 为 nil 或全部满足时返回 nil。bins 与 runtime 探测并行执行（zsh 登录
// shell 探测单次可达秒级，串行累加曾拖慢全库重扫），告警仍按声明顺序返回。
func verifyDependencies(r *Requires) []string {
	if r == nil {
		return nil
	}
	bins := make([]string, 0, len(r.Bins))
	for _, bin := range r.Bins {
		bin = strings.TrimSpace(bin)
		if bin != "" {
			bins = append(bins, bin)
		}
	}

	// 并行探测（顺序无关；告警按声明顺序回填）
	binResults := make([]error, len(bins))
	var wg sync.WaitGroup
	for i, bin := range bins {
		wg.Add(1)
		go func(i int, bin string) {
			defer wg.Done()
			_, err := resolveBinCached(bin)
			binResults[i] = err
		}(i, bin)
	}
	rtResults := make([]error, len(r.Runtime))
	keys := make([]string, 0, len(r.Runtime))
	for runtime := range r.Runtime {
		keys = append(keys, runtime)
	}
	for i := range keys {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rtResults[i] = checkRuntimeVersionCached(keys[i], r.Runtime[keys[i]])
		}(i)
	}
	wg.Wait()

	var warnings []string
	for i, bin := range bins {
		if binResults[i] != nil {
			warnings = append(warnings, fmt.Sprintf("必需的二进制文件 %q 未找到", bin))
		}
	}
	for _, key := range r.Env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if os.Getenv(key) == "" {
			warnings = append(warnings, fmt.Sprintf("必需的环境变量 %q 未设置", key))
		}
	}
	for i := range keys {
		if rtResults[i] != nil {
			warnings = append(warnings, rtResults[i].Error())
		}
	}
	return warnings
}

// resolveBinCached 带进程级缓存的 resolveBin。
func resolveBinCached(name string) (string, error) {
	key := "bin:" + name
	if v, ok := depCache.Load(key); ok {
		c := v.(cachedDepResult)
		return c.path, c.verr
	}
	path, err := resolveBin(name)
	depCache.Store(key, cachedDepResult{path: path, verr: err})
	return path, err
}

// checkRuntimeVersionCached 带进程级缓存的 checkRuntimeVersion（按运行时+约束缓存）。
func checkRuntimeVersionCached(runtime, constraint string) error {
	key := "rt:" + runtime + ":" + constraint
	if v, ok := depCache.Load(key); ok {
		return v.(cachedDepResult).rtErr
	}
	err := checkRuntimeVersion(runtime, constraint)
	depCache.Store(key, cachedDepResult{rtErr: err})
	return err
}

// resolveBin 三层查找链解析二进制真实路径。
//
//  1. 先用 exec.LookPath 查进程 PATH（快，但 macOS GUI 进程 PATH 极其受限）；
//  2. 再用 shell 解析——zsh -c 'which <bin>' 会 source ~/.zprofile / ~/.zshrc，
//     拿到用户完整 PATH（nvm、bun、volta、Homebrew 等安装位置都能覆盖）；
//  3. 最后扫常见安装目录兜底，直接读文件系统（不依赖任何 PATH）。
//
// 成功返回绝对路径，全部失败返回 error。
func resolveBin(name string) (string, error) {
	// Layer 1: 进程 PATH
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	// Layer 2: shell 解析（zsh 会 source 用户 profile）
	if p, err := resolveBinViaShell(name); err == nil {
		return p, nil
	}

	// Layer 3: 常见安装目录直接扫描
	if p, err := resolveBinInWellKnownDirs(name); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("未找到 %q", name)
}

// resolveBinViaShell 通过 zsh -c 'which <bin>' 解析二进制路径。
// zsh 作为登录 shell 启动时会 source ~/.zprofile，拿到用户完整 PATH
// （包含 nvm / bun / volta / Homebrew / 手动 export 的路径等），
// 这是 macOS GUI 进程 PATH 阉割情况下最关键的补充层。
// 超时保护：shell 启动 + profile 解析不应超过 2 秒。
func resolveBinViaShell(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// -l: login shell，自动 source ~/.zprofile
	// -i: interactive，额外 source ~/.zshrc 拿到用户手动 export 的 PATH
	// -c: 执行命令
	// Setsid: 脱离控制终端。交互式 zsh 会主动打开 /dev/tty 并开启作业控制
	// （tcsetpgrp 把自己设为前台进程组），把父进程（TUI）永久挤到后台，
	// 导致后续 bubbletea 进入 raw mode 时触发 SIGTTOU 被挂起。
	// 新会话中没有控制终端，zsh 便不会开启作业控制，PATH 补全能力不受影响。
	cmd := exec.CommandContext(ctx, "zsh", "-lic", "which "+name)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("zsh which 失败: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("zsh which 返回空")
	}
	// 确认路径存在且可执行
	if info, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("路径 %q 不存在: %w", path, err)
	} else if info.IsDir() {
		return "", fmt.Errorf("路径 %q 是目录而非文件", path)
	} else if info.Mode()&0111 == 0 {
		return "", fmt.Errorf("路径 %q 不可执行", path)
	}
	return path, nil
}

// resolveBinInWellKnownDirs 在常见安装目录中直接查找二进制。
// 覆盖 Homebrew（Intel / Apple Silicon）、nvm 默认路径、bun、volta、
// Rust cargo、Go binary、Android SDK、Android NDK 以及用户自定义 bin 等。
// 这是最后的兜底层——不依赖 PATH 也不依赖 shell，直接扫文件系统。
func resolveBinInWellKnownDirs(name string) (string, error) {
	home, _ := os.UserHomeDir()
	candidates := []string{
		// Homebrew
		"/opt/homebrew/bin", // Apple Silicon
		"/usr/local/bin",    // Intel + 通用
		"/opt/homebrew/sbin",
		"/usr/local/sbin",
		// bun
		filepath.Join(home, ".bun", "bin"),
		// volta
		filepath.Join(home, ".volta", "bin"),
		// cargo (Rust)
		filepath.Join(home, ".cargo", "bin"),
		// go binary
		filepath.Join(home, "go", "bin"),
		// Android SDK / NDK
		"$ANDROID_HOME/platform-tools",
		"$ANDROID_HOME/cmdline-tools/latest/bin",
		"$ANDROID_NDK_HOME",
		// 用户自定义
		filepath.Join(home, ".local", "bin"),
		"/usr/bin",
		"/bin",
		// nvm —— 版本目录是动态的，这里用 glob 扫
	}

	for _, dir := range candidates {
		if strings.HasPrefix(dir, "$") {
			// 环境变量展开
			env := strings.TrimPrefix(strings.TrimSuffix(dir, "/platform-tools"), "$")
			dir = os.Getenv(env)
			if dir == "" {
				continue
			}
			if strings.Contains(candidates[len(candidates)-3], "$platform-tools") {
				dir = filepath.Join(dir, "platform-tools")
			} else if strings.Contains(candidates[len(candidates)-2], "$cmdline-tools") {
				dir = filepath.Join(dir, "cmdline-tools", "latest", "bin")
			}
		}
		full := filepath.Join(dir, name)
		if info, err := os.Stat(full); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return full, nil
		}
	}

	// nvm: ~/.nvm/versions/node/*/bin/<name>
	nvmGlob := filepath.Join(home, ".nvm", "versions", "node", "*", "bin", name)
	if matches, err := filepath.Glob(nvmGlob); err == nil {
		for _, full := range matches {
			if info, err := os.Stat(full); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
				return full, nil
			}
		}
	}

	return "", fmt.Errorf("常见目录未找到 %q", name)
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

	// 用 resolveBin 三层查找链找二进制真实路径
	resolved, err := resolveBin(bin)
	if err != nil {
		return fmt.Errorf("需要运行时 %q（约束 %q），但未找到 %q。请先安装 %s", runtime, constraint, bin, runtime)
	}

	// 用真实路径读版本号
	ver, err := readRuntimeVersion(resolved)
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
	switch filepath.Base(bin) {
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

	// 超时保护：个别运行时（如 dotnet 首次冷启动）可能挂起数秒，拖死依赖检查
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("执行 %q 超时", bin)
	}
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
