package skillstore

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// QualityIssue 是技能质量检查的单条发现。
// Severity: "error"（必须修复，硬错误/断裂引用）| "warning"（建议修复，软化词/体积）。
type QualityIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
}

// QualityReport 是 CheckQuality 的输出，JSON 序列化后即 CLI 报告。
type QualityReport struct {
	Path   string         `json:"path"`
	OK     bool           `json:"ok"`
	Stats  QualityStats   `json:"stats"`
	Issues []QualityIssue `json:"issues"`
}

// QualityStats 汇总确定性统计量。
type QualityStats struct {
	Lines     int `json:"lines"`
	SoftRules int `json:"soft_rules"`
	Refs      int `json:"refs"`
}

// 软化词模式：识别待硬化规则（HASP 方法）。
// Go RE2 不支持负向前瞻，"建议"的后缀白名单在扫描时单独排除。
var softPatterns = []struct {
	re    *regexp.Regexp
	label string
}{
	{regexp.MustCompile(`可以考虑|根据情况|是可选的`), "软化表述"},
	{regexp.MustCompile(`建议`), "建议句式"},
}

// 建议句式的后缀白名单（"建议升级/安装/使用"是正常指令，非软化）。
var suggestWhitelist = regexp.MustCompile(`建议\s*(?:升级|安装|使用|先|先通过)`)

// refFormatRe 校验引文标记格式：[ref:ID] 的 ID 必须 ref_ 前缀。
var (
	refRe       = regexp.MustCompile(`\[ref:([A-Za-z0-9_\-]+)\]`)
	mdLinkRe    = regexp.MustCompile(`\(([^)\s]+\.md)\)`)
	triggerSema = []string{"时使用", "时调用", "触发"}
)

const (
	maxDescChars = 1024
	maxBodyLines = 150
)

// CheckQuality 对任意技能目录（全局库或项目级 .agents/skills/<name>）执行
// 确定性质量检查。本函数是 SKILL.md 规范机器校验的唯一实现——
// skill-creator 的审查流程与 retrospect 的质量门均调用本能力（经 CLI），
// 禁止在技能文档内复写检查逻辑，防止实现漂移。
func CheckQuality(dir string) (*QualityReport, error) {
	rep := &QualityReport{Path: dir, OK: true, Issues: []QualityIssue{}}
	skillPath := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", skillPath, err)
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	rep.Stats.Lines = len(lines)

	// 1. 结构解析（装载级硬错误）
	sk, _, loadErr := LoadSkillFromDir(dir, "filesystem")
	if loadErr != nil {
		rep.Issues = append(rep.Issues, QualityIssue{
			Severity: "error", Code: "frontmatter",
			Message: fmt.Sprintf("frontmatter 解析失败: %v", loadErr),
		})
	}
	if sk != nil {
		if err := ValidateSkillName(sk.Name); err != nil {
			rep.Issues = append(rep.Issues, QualityIssue{
				Severity: "error", Code: "name", Message: err.Error(),
			})
		}
		if err := ValidateSkillDescription(sk.Description); err != nil {
			rep.Issues = append(rep.Issues, QualityIssue{
				Severity: "error", Code: "description", Message: err.Error(),
			})
		}
		// 触发语义：description 须说明"何时使用"（YAML 折叠块对全文检查）
		hit := false
		for _, sema := range triggerSema {
			if strings.Contains(sk.Description, sema) || strings.Contains(text[:min(len(text), 1200)], sema) {
				hit = true
				break
			}
		}
		if !hit {
			rep.Issues = append(rep.Issues, QualityIssue{
				Severity: "warning", Code: "trigger-semantic",
				Message: `description 未说明"何时使用/时调用/触发"（触发路由信号缺失）`,
			})
		}
	}

	// 2. 软化词扫描（逐行，"建议"检查后缀白名单）
	for i, line := range lines {
		for _, p := range softPatterns {
			if p.re.MatchString(line) {
				if p.label == "建议句式" && suggestWhitelist.MatchString(line) {
					continue
				}
				rep.Issues = append(rep.Issues, QualityIssue{
					Severity: "warning", Code: "soft-rule", Line: i + 1,
					Message: fmt.Sprintf("%s：%s", p.label, strings.TrimSpace(line)[:min(80, len(strings.TrimSpace(line)))]),
				})
				break
			}
		}
	}
	rep.Stats.SoftRules = countCode(rep.Issues, "soft-rule")

	// 3. 体积
	if len(lines) > maxBodyLines {
		rep.Issues = append(rep.Issues, QualityIssue{
			Severity: "warning", Code: "size",
			Message: fmt.Sprintf("正文 %d 行超过建议上限 %d 行（复盘蒸馏的候选技能应短小聚焦）", len(lines), maxBodyLines),
		})
	}

	// 4. 引文格式：[ref:ID] 的 ID 必须 ref_ 前缀
	for i, line := range lines {
		for _, ref := range refRe.FindAllStringSubmatch(line, -1) {
			rep.Stats.Refs++
			if !strings.HasPrefix(ref[1], "ref_") {
				rep.Issues = append(rep.Issues, QualityIssue{
					Severity: "error", Code: "ref-format", Line: i + 1,
					Message: fmt.Sprintf("引文标记 %q 的 ID 应以 ref_ 开头", ref[0]),
				})
			}
		}
	}

	// 5. 正文 .md 链接可达性（相对路径，跳过 http）
	for i, line := range lines {
		for _, m := range mdLinkRe.FindAllStringSubmatch(line, -1) {
			link := strings.TrimPrefix(m[1], "./")
			if strings.HasPrefix(link, "http") || !strings.HasSuffix(strings.ToLower(link), ".md") {
				continue
			}
			link = strings.ReplaceAll(link, "{base_dir}/", "")
			if _, err := os.Stat(filepath.Join(dir, link)); err != nil {
				rep.Issues = append(rep.Issues, QualityIssue{
					Severity: "error", Code: "broken-link", Line: i + 1,
					Message: fmt.Sprintf("引用的文件不存在: %s", m[1]),
				})
			}
		}
	}

	for _, iss := range rep.Issues {
		if iss.Severity == "error" {
			rep.OK = false
			break
		}
	}
	return rep, nil
}

func countCode(issues []QualityIssue, code string) int {
	n := 0
	for _, iss := range issues {
		if iss.Code == code {
			n++
		}
	}
	return n
}
