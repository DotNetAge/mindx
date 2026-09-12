package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── market parent ─────────────────────────────────────────────

var marketCmd = &cobra.Command{
	Use:   "market",
	Short: "Market commands: search & install packages, plus packaging toolchain",
	Long: `Market commands in two groups:

Consumer commands (requires mindx start) — browse the online market and
install agent/skill packages with sha256 verification:

  mindx market list [--kind agent|skill] [--filter <keyword>] [--json]
  mindx market install <agent|skill> <name> [--overwrite]

Packaging toolchain (no daemon required) — build .mindpkg bundles for the
COS static market:

Source layout is <src>/<category>/<name>/ (canonical directory format with IDENTITY.md + SOUL.md;
category dirs: contents/data/dev/finance/marketing/office). Legacy single-file sources
(<category>/<name>.md) are auto-migrated at pack time and can be canonicalized in place
with 'mindx market canonicalize-agents'.

Examples:
  mindx market canonicalize-agents --src ~/workspaces/ai-ecosystem/mindx-market/agents
  mindx market pack-agents --src ~/workspaces/ai-ecosystem/mindx-market/agents --out dist`,
}

func init() {
	marketListCmd.Flags().Bool("json", false, "Output structured JSON (requires mindx start)")
	marketListCmd.Flags().String("kind", "", "Filter by package type: agent or skill")
	marketListCmd.Flags().StringSliceP("filter", "f", nil, "Filter packages by keyword, matched against name/description/role/category/skills (case-insensitive, comma-separated)")
	marketInstallCmd.Flags().Bool("overwrite", false, "Overwrite existing target (agent dir or skill with the same name)")
	marketInstallCmd.Flags().Bool("auto", false, "Auto-install missing dependencies without prompting")
	marketInstallCmd.Flags().Bool("skip-deps", false, "Skip dependency checking and installation")

	marketCmd.AddCommand(marketListCmd, marketInstallCmd, marketCanonicalizeAgentsCmd, marketPackAgentsCmd)
	rootCmd.AddCommand(marketCmd)
}

// ── market list：市场货架检索（消费端，需 daemon） ─────────────

var marketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List packages available in the online market (requires mindx start)",
	Long: `Fetch the market manifest and list agent/skill packages.

Output includes kind (agent/skill), name, category, role, description and bundled skills.
Use --kind to filter by package type and --filter to match by keyword
(case-insensitive, against name/description/role/category/skills).

Examples:
  mindx market list
  mindx market list --kind agent
  mindx market list --kind skill --filter "架构"
  mindx market list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")
		kind, _ := cmd.Flags().GetString("kind")
		filters, _ := cmd.Flags().GetStringSlice("filter")

		if kind != "" && kind != "agent" && kind != "skill" {
			return fmt.Errorf("--kind 仅支持 agent 或 skill")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MarketList()
		if err != nil {
			return err
		}

		// market.list 返回 {packages, source, warning, updated_at}；包条目为
		// camelCase DTO。过滤在客户端完成，与 agent list 的分层一致。
		var res struct {
			Packages  []marketPackage `json:"packages"`
			Source    string          `json:"source"`
			Warning   string          `json:"warning"`
			UpdatedAt string          `json:"updated_at"`
		}
		if err := json.Unmarshal(result, &res); err != nil {
			// 兜底：无法解析时原样输出
			fmt.Println(string(result))
			return nil
		}

		filtered := make([]marketPackage, 0, len(res.Packages))
		for _, p := range res.Packages {
			if kind != "" && p.Kind != kind {
				continue
			}
			if !marketPackageMatch(p, filters) {
				continue
			}
			filtered = append(filtered, p)
		}

		if useJSON {
			out := map[string]any{
				"packages":   filtered,
				"source":     res.Source,
				"warning":    res.Warning,
				"updated_at": res.UpdatedAt,
			}
			formatted, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(formatted))
			return nil
		}

		if len(filtered) == 0 {
			fmt.Println("市场中没有匹配的分发包。")
			fmt.Println("提示：调整 --filter 关键词，或运行 `mindx market list` 查看全部。")
			return nil
		}

		table := render.NewTable([]string{"Kind", "Name", "Category", "Role", "Description", "Skills"}, 100)
		for _, p := range filtered {
			role := p.Role
			if role == "" {
				role = "—"
			}
			desc := p.Description
			if desc == "" {
				desc = "—"
			}
			table.AddRow([]string{p.Kind, p.Name, p.Category, role, desc, strings.Join(p.Skills, ", ")})
		}
		fmt.Println(table.Render())

		agents, skills := 0, 0
		for _, p := range filtered {
			if p.Kind == "agent" {
				agents++
			} else {
				skills++
			}
		}
		fmt.Printf("\n%d package(s)（agent %d / skill %d）", len(filtered), agents, skills)
		if res.Source != "" {
			fmt.Printf(" · source: %s", res.Source)
		}
		fmt.Println()
		if res.Warning != "" {
			fmt.Printf("⚠️  %s\n", res.Warning)
		}
		return nil
	},
}

// marketPackage 是 market.list 响应中包条目的 CLI 视图（camelCase 对齐 daemon DTO）。
type marketPackage struct {
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Role        string   `json:"role"`
	Category    string   `json:"category"`
	Skills      []string `json:"skills"`
	Version     string   `json:"version"`
	Size        int64    `json:"size"`
}

// marketPackageMatch 判断包条目是否命中任一关键词（大小写不敏感，匹配
// name/description/role/category/skills；关键词为空视为不过滤）。
func marketPackageMatch(p marketPackage, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		p.Name, p.Description, p.Role, p.Category, strings.Join(p.Skills, " "),
	}, " "))
	for _, f := range filters {
		if f = strings.TrimSpace(strings.ToLower(f)); f != "" && strings.Contains(haystack, f) {
			return true
		}
	}
	return false
}

// ── market install：市场分发包安装（消费端，需 daemon） ────────

var marketInstallCmd = &cobra.Command{
	Use:   "install <agent|skill> <name>",
	Short: "Install a package from the market (requires mindx start)",
	Long: `Download a package from the market (sha256 verified) and install it.

Agent packages land in ~/.mindx/agents/<name>/ (unhired by default — run
'mindx agent hire <name>' to enable it for sessions). Skill packages land
in the global skill library.

Examples:
  mindx market install agent architect
  mindx market install skill api-design
  mindx market install agent architect --overwrite`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		kind, name := args[0], args[1]
		if kind != "agent" && kind != "skill" {
			return fmt.Errorf("kind 必须为 agent 或 skill")
		}
		overwrite, _ := cmd.Flags().GetBool("overwrite")
		skipDeps, _ := cmd.Flags().GetBool("skip-deps")
		autoInstall, _ := cmd.Flags().GetBool("auto")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MarketInstall(kind, name, overwrite, skipDeps, autoInstall)
		if err != nil {
			return err
		}

		var pretty map[string]any
		if err := json.Unmarshal(result, &pretty); err == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
		} else {
			fmt.Println(string(result))
		}

		// 市场分发的 Agent 模板不带 hired 字段（默认未雇佣），提示雇佣入口
		if kind == "agent" {
			fmt.Printf("提示：已安装智能体默认未雇佣，执行 `mindx agent hire %s` 后即可用于会话。\n", name)
		}
		return nil
	},
}

// agentSource 是一个待打包的 Agent 源条目。
type agentSource struct {
	category string // 一级子目录名（根目录散落文件为空 = 未分类）
	path     string // 目录（规范化格式）或单文件（旧格式）的绝对路径
	base     string // 条目名（去掉 .md 后缀；目录名原样）
	isDir    bool   // true = 规范化目录格式（含 IDENTITY.md）
}

// ── canonicalize-agents：源仓库就地规范化 ─────────────────────

var marketCanonicalizeAgentsCmd = &cobra.Command{
	Use:   "canonicalize-agents",
	Short: "Migrate legacy single-file agents in place to the canonical directory format",
	RunE:  runMarketCanonicalizeAgents,
}

var canonSrc string

func init() {
	marketCanonicalizeAgentsCmd.Flags().StringVar(&canonSrc, "src", "", "Agent 源目录（<分类>/<name>.md 或 <分类>/<name>/）")
}

func runMarketCanonicalizeAgents(cmd *cobra.Command, args []string) error {
	srcDir := strings.TrimSpace(canonSrc)
	if srcDir == "" {
		return fmt.Errorf("必须通过 --src 指定 Agent 源目录")
	}

	// 收集源条目（与打包共用同一收集逻辑）
	sources, err := collectAgentSources(srcDir)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("源目录 %s 中没有可处理的 Agent（.md 文件或规范化目录）", srcDir)
	}

	// 单文件：先归一化（剔除第三方遗留字段、正文头部杂散分隔线）再迁移
	normalized := 0
	migrateDirs := make(map[string]bool) // 需要 MigrateLegacyFiles 的父目录
	for _, s := range sources {
		if s.isDir {
			continue // 规范化目录已就位，无需处理
		}
		data, err := os.ReadFile(s.path)
		if err != nil {
			return fmt.Errorf("读取 %s 失败: %w", s.path, err)
		}
		content, changed := normalizeLegacyAgent(string(data))
		if changed {
			normalized++
			fmt.Printf("已归一化: %s（清除第三方遗留字段 / 正文头部杂散分隔线）\n", s.path)
		}
		if err := os.WriteFile(s.path, []byte(content), 0644); err != nil {
			return fmt.Errorf("回写 %s 失败: %w", s.path, err)
		}
		migrateDirs[filepath.Dir(s.path)] = true
	}

	// 逐目录执行官方迁移语义：旧单文件 → 目录化（frontmatter → IDENTITY.md，正文 → SOUL.md，
	// 原文件备份为 .bak）。迁移在源目录内进行，目录条目不受影响。
	migrated := 0
	for dir := range migrateDirs {
		for file, err := range agentstore.MigrateLegacyFiles(dir) {
			if err != nil {
				return fmt.Errorf("迁移 %s 失败: %w", file, err)
			}
			migrated++
			fmt.Printf("已目录化: %s/%s\n", dir, strings.TrimSuffix(file, ".md"))
		}
	}

	already := len(sources) - countFiles(sources)
	fmt.Printf("完成: 共 %d 个 Agent（本次目录化 %d 个，归一化 %d 个，已就位 %d 个）；原单文件已备份为 .bak，确认无误后可自行删除。\n",
		len(sources), migrated, normalized, already)
	return nil
}

// countFiles 统计单文件条目数。
func countFiles(sources []agentSource) int {
	n := 0
	for _, s := range sources {
		if !s.isDir {
			n++
		}
	}
	return n
}

// ── pack-agents：打包为 .mindpkg ─────────────────────────────

var marketPackAgentsCmd = &cobra.Command{
	Use:   "pack-agents",
	Short: "Pack agents into .mindpkg bundles with install round-trip verification",
	RunE:  runMarketPackAgents,
}

var (
	packSrc    string
	packOut    string
	packSkills string
)

func init() {
	marketPackAgentsCmd.Flags().StringVar(&packSrc, "src", "", "Agent 源目录（<分类>/<name>/ 或 <分类>/<name>.md）")
	marketPackAgentsCmd.Flags().StringVar(&packOut, "out", "dist", "分发包输出目录")
	marketPackAgentsCmd.Flags().StringVar(&packSkills, "skills", "", "全局技能库目录（其下为 <skill-name>/SKILL.md 平铺结构），多个库用英文逗号分隔；Agent 声明引用的全局技能将按库副本打入包内")
}

func runMarketPackAgents(cmd *cobra.Command, args []string) error {
	srcDir := strings.TrimSpace(packSrc)
	if srcDir == "" {
		return fmt.Errorf("必须通过 --src 指定 Agent 源目录")
	}
	outDir := strings.TrimSpace(packOut)
	if outDir == "" {
		return fmt.Errorf("输出目录不能为空")
	}

	// ① 收集源条目并做跨分类重名预检：市场货架以 kind+name 为唯一键
	sources, err := collectAgentSources(srcDir)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("源目录 %s 中没有可打包的 Agent（.md 文件或规范化目录）", srcDir)
	}
	seen := make(map[string]agentSource, len(sources))
	for _, s := range sources {
		key := strings.ToLower(s.base)
		if prev, dup := seen[key]; dup {
			return fmt.Errorf("Agent 重名：%s 与 %s（分类 %s / %s），货架以名称为唯一键，请先改名",
				s.path, prev.path, s.category, prev.category)
		}
		seen[key] = s
	}

	// ② 暂存目录：规范化目录整树拷贝，旧单文件归一化后平铺，交由官方迁移语义统一转换
	staging, err := os.MkdirTemp("", "mindx-pack-agents-*")
	if err != nil {
		return fmt.Errorf("创建暂存目录失败: %w", err)
	}
	defer os.RemoveAll(staging)

	for _, s := range sources {
		if s.isDir {
			// os.CopyFS 要求目标不存在，staging 全新目录满足
			if err := os.CopyFS(filepath.Join(staging, s.base), os.DirFS(s.path)); err != nil {
				return fmt.Errorf("拷贝 Agent 目录 %s 失败: %w", s.path, err)
			}
			continue
		}
		data, err := os.ReadFile(s.path)
		if err != nil {
			return fmt.Errorf("读取 %s 失败: %w", s.path, err)
		}
		content, _ := normalizeLegacyAgent(string(data))
		if err := os.WriteFile(filepath.Join(staging, s.base+".md"), []byte(content), 0644); err != nil {
			return fmt.Errorf("写入暂存文件失败: %w", err)
		}
	}
	if report := agentstore.MigrateLegacyFiles(staging); len(report) > 0 {
		for file, err := range report {
			if err != nil {
				return fmt.Errorf("迁移 %s 失败: %w", file, err)
			}
		}
	}
	store, loadErrs, err := agentstore.Load(staging)
	if err != nil {
		return fmt.Errorf("加载暂存目录失败: %w", err)
	}
	for name, err := range loadErrs {
		if err != nil {
			return fmt.Errorf("加载 Agent %s 失败: %w", name, err)
		}
	}

	// ③ 技能库：市场源 Agent 声明的全局引用技能需按库副本打入包内（保真）；
	// 未指定 --skills 时退回空库兜底（声明引用将以警告形式暴露）
	skillStore, err := buildPackSkillStore(packSkills, staging)
	if err != nil {
		return err
	}

	// ④ 逐个打包并做安装回环验证
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}
	verifyRoot, err := os.MkdirTemp("", "mindx-pack-verify-*")
	if err != nil {
		return fmt.Errorf("创建验证目录失败: %w", err)
	}
	defer os.RemoveAll(verifyRoot)

	agents := store.List()
	packed, failed := 0, 0
	for _, a := range agents {
		if _, ok := seen[strings.ToLower(a.Meta.Name)]; !ok {
			return fmt.Errorf("Agent %s 未找到源文件映射", a.Meta.Name)
		}
		pkgPath := filepath.Join(outDir, a.Meta.Name+".mindpkg")
		manifest, warnings, err := bundle.ExportAgent(a, skillStore, pkgPath)
		if err != nil {
			failed++
			fmt.Printf("打包失败: %s: %v\n", a.Meta.Name, err)
			continue
		}
		// 安装回环验证：确保每个包都能被真实安装器展开落位
		result, err := bundle.Install(pkgPath, bundle.InstallOptions{
			AgentsDir: filepath.Join(verifyRoot, "agents"),
			GlobalDir: filepath.Join(verifyRoot, "skills"),
		})
		if err != nil {
			failed++
			fmt.Printf("安装验证失败: %s: %v\n", a.Meta.Name, err)
			continue
		}
		if result.Name != manifest.Name || string(result.Kind) != string(manifest.Kind) {
			failed++
			fmt.Printf("安装验证不一致: %s（清单 %s/%s，落位 %s/%s）\n",
				a.Meta.Name, manifest.Kind, manifest.Name, result.Kind, result.Name)
			continue
		}
		packed++
		info, _ := os.Stat(pkgPath)
		sizeKB := "0"
		if info != nil {
			sizeKB = fmt.Sprintf("%.1f", float64(info.Size())/1024)
		}
		line := fmt.Sprintf("已打包: %s/%s（%s KB）→ %s", manifest.Category, manifest.Name, sizeKB, pkgPath)
		if len(warnings) > 0 {
			line += fmt.Sprintf("（警告: %s）", strings.Join(warnings, "; "))
		}
		fmt.Println(line)
	}
	if failed > 0 {
		return fmt.Errorf("共 %d 个 Agent，成功 %d 个，失败 %d 个", len(agents), packed, failed)
	}
	fmt.Printf("全部完成: 共 %d 个 Agent 打包并通过安装验证，输出目录: %s\n", packed, outDir)
	return nil
}

// buildPackSkillStore 构建打包用技能库：--skills 支持逗号分隔多个平铺库，首个库
// 经 NewStore 装载，其余库依序并入全局注册表（声明引用的全局技能按库副本打进包
// 内）；未指定时用暂存目录下的空库兜底。任一库加载失败都直接报错，避免缺技能的
// 包静默出厂。
func buildPackSkillStore(skillsDirs, stagingDir string) (*skillstore.Store, error) {
	dirs := strings.Split(skillsDirs, ",")
	var store *skillstore.Store
	for _, raw := range dirs {
		dir := strings.TrimSpace(raw)
		if dir == "" {
			continue
		}
		if store == nil {
			s, err := skillstore.NewStore(dir, "")
			if err != nil {
				return nil, fmt.Errorf("加载技能库 %s 失败: %w", dir, err)
			}
			store = s
			continue
		}
		if err := store.MergeGlobalDir(dir); err != nil {
			return nil, fmt.Errorf("并入技能库 %s 失败: %w", dir, err)
		}
	}
	if store != nil {
		return store, nil
	}
	store, err := skillstore.NewStore(filepath.Join(stagingDir, "_empty_skills"), "")
	if err != nil {
		return nil, fmt.Errorf("创建空技能库失败: %w", err)
	}
	return store, nil
}

// ── 共享：源条目收集与归一化 ─────────────────────────────────

// collectAgentSources 收集 Agent 源条目：一级子目录即分类，根目录散落条目视为未分类。
// 条目可为规范化目录（<name>/IDENTITY.md…）或旧单文件（<name>.md），忽略隐藏与 .bak。
func collectAgentSources(srcDir string) ([]agentSource, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("读取源目录失败: %w", err)
	}
	var sources []agentSource
	add := func(category, path, name string, isDir bool) {
		sources = append(sources, agentSource{
			category: category,
			path:     path,
			base:     strings.TrimSuffix(name, ".md"),
			isDir:    isDir,
		})
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".bak") {
			continue
		}
		p := filepath.Join(srcDir, name)
		if e.IsDir() {
			subEntries, err := os.ReadDir(p)
			if err != nil {
				return nil, fmt.Errorf("读取分类目录 %s 失败: %w", p, err)
			}
			for _, se := range subEntries {
				sn := se.Name()
				if strings.HasPrefix(sn, ".") || strings.HasSuffix(sn, ".bak") {
					continue
				}
				sp := filepath.Join(p, sn)
				if se.IsDir() {
					// 规范化目录格式：<分类>/<name>/
					if _, err := os.Stat(filepath.Join(sp, "IDENTITY.md")); err != nil {
						continue // 无 IDENTITY.md 的附属目录（如素材）跳过
					}
					add(name, sp, sn, true)
					continue
				}
				if strings.HasSuffix(sn, ".md") {
					add(name, sp, sn, false)
				}
			}
			continue
		}
		if strings.HasSuffix(name, ".md") {
			add("", p, name, false)
		}
	}
	return sources, nil
}

// normalizeLegacyAgent 清洗第三方遗留源文件：
//   - frontmatter 中的 model / tools 字段为 Claude 生态遗留（mindx 模型绑定是环境相关的，
//     外部模型名会导致无默认模型的用户 createRuntime 失败；tools 是白名单语义，与
//     mindx 的 exclude_tools 黑名单不可映射），直接剔除；
//   - 正文头部紧随 frontmatter 的杂散 "---" 分隔线（源文件 frontmatter 收尾手误）剥掉。
//
// 返回清洗后的内容与是否发生改动。
func normalizeLegacyAgent(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	changed := false

	// 定位 frontmatter 边界：首行 --- 与其后第一个 --- 行
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, false
	}
	frontEnd := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			frontEnd = i
			break
		}
	}
	if frontEnd < 0 {
		return content, false
	}

	// 剔除 frontmatter 中的第三方遗留字段行（只匹配无缩进的顶层键，避免误删 meta 块内嵌套字段）
	front := make([]string, 0, frontEnd)
	for _, line := range lines[1:frontEnd] {
		if strings.HasPrefix(line, "model:") || strings.HasPrefix(line, "tools:") {
			changed = true
			continue
		}
		front = append(front, line)
	}

	// 剥掉正文头部的杂散 --- 行（仅正文起始处，不影响中段水平分隔线）
	body := lines[frontEnd+1:]
	for len(body) > 0 && strings.TrimSpace(body[0]) == "---" {
		body = body[1:]
		changed = true
	}

	if !changed {
		return content, false
	}
	out := []string{"---"}
	out = append(out, front...)
	out = append(out, "---")
	out = append(out, body...)
	return strings.Join(out, "\n"), true
}
