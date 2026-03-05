package skills

import (
	"mindx/internal/entity"
	"sort"
	"strings"
	"sync"
)

// KeywordIndex 关键词索引（MVP 简化版）
// 用于快速匹配 Skills，不使用向量化
type KeywordIndex struct {
	mu sync.RWMutex
	// keyword -> []skillName 的倒排索引
	index map[string][]string
	// skillName -> SkillDef 的映射
	skills map[string]*entity.SkillDef
}

// NewKeywordIndex 创建关键词索引
func NewKeywordIndex() *KeywordIndex {
	return &KeywordIndex{
		index:  make(map[string][]string),
		skills: make(map[string]*entity.SkillDef),
	}
}

// IndexSkill 索引单个 Skill
func (idx *KeywordIndex) IndexSkill(def *entity.SkillDef) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	skillName := def.Name

	// 保存 Skill 定义
	idx.skills[skillName] = def

	// 索引 Tags（作为关键词）
	for _, tag := range def.Tags {
		keyword := strings.ToLower(strings.TrimSpace(tag))
		if keyword == "" {
			continue
		}

		// 添加到倒排索引
		if !contains(idx.index[keyword], skillName) {
			idx.index[keyword] = append(idx.index[keyword], skillName)
		}
	}

	// 索引 Name（分词）
	nameKeywords := tokenize(def.Name)
	for _, keyword := range nameKeywords {
		if !contains(idx.index[keyword], skillName) {
			idx.index[keyword] = append(idx.index[keyword], skillName)
		}
	}

	// 索引 Description（分词）
	descKeywords := tokenize(def.Description)
	for _, keyword := range descKeywords {
		if !contains(idx.index[keyword], skillName) {
			idx.index[keyword] = append(idx.index[keyword], skillName)
		}
	}
}

// Search 搜索匹配的 Skills
func (idx *KeywordIndex) Search(keywords []string, topK int) []*SkillMatch {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if topK <= 0 {
		topK = 3
	}

	// 统计每个 Skill 的匹配分数
	scores := make(map[string]float64)

	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}

		// 精确匹配
		if skillNames, ok := idx.index[keyword]; ok {
			for _, skillName := range skillNames {
				scores[skillName] += 1.0
			}
		}

		// 模糊匹配（包含关系）
		for indexedKeyword, skillNames := range idx.index {
			if strings.Contains(indexedKeyword, keyword) || strings.Contains(keyword, indexedKeyword) {
				for _, skillName := range skillNames {
					scores[skillName] += 0.5
				}
			}
		}
	}

	// 转换为 SkillMatch 列表
	matches := make([]*SkillMatch, 0, len(scores))
	for skillName, score := range scores {
		if def, ok := idx.skills[skillName]; ok {
			matches = append(matches, &SkillMatch{
				Name:  skillName,
				Def:   def,
				Score: score,
			})
		}
	}

	// 按分数排序
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	// 返回 TopK
	if len(matches) > topK {
		matches = matches[:topK]
	}

	return matches
}

// GetSkill 获取单个 Skill 定义
func (idx *KeywordIndex) GetSkill(name string) (*entity.SkillDef, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	def, ok := idx.skills[name]
	return def, ok
}

// GetAllSkills 获取所有 Skills
func (idx *KeywordIndex) GetAllSkills() []*entity.SkillDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	skills := make([]*entity.SkillDef, 0, len(idx.skills))
	for _, def := range idx.skills {
		skills = append(skills, def)
	}

	return skills
}

// Clear 清空索引
func (idx *KeywordIndex) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.index = make(map[string][]string)
	idx.skills = make(map[string]*entity.SkillDef)
}

// SkillMatch Skill 匹配结果
type SkillMatch struct {
	Name  string
	Def   *entity.SkillDef
	Score float64
}

// tokenize 分词（简单实现）
func tokenize(text string) []string {
	text = strings.ToLower(text)

	// 按空格、下划线、连字符分割
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		"/", " ",
	)
	text = replacer.Replace(text)

	// 分割并过滤空字符串
	tokens := strings.Fields(text)

	// 去重
	seen := make(map[string]bool)
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if !seen[token] && len(token) > 1 { // 忽略单字符
			seen[token] = true
			result = append(result, token)
		}
	}

	return result
}

// contains 检查字符串切片是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
