package processors

import (
	"context"
	"mindx/internal/entity"
	"mindx/pkg/logging"
)

// SkillSearcher 技能搜索接口
type SkillSearcher interface {
	Search(query string, topK int) ([]*entity.SkillMatch, error)
}

// ToolAssembler 工具组装接口
type ToolAssembler interface {
	AssembleTools(skill *entity.Skill) ([]entity.ToolSchema, error)
}

// SkillMatchProcessor 技能匹配处理器
// 职责：基于意图关键词匹配技能，并组装所需工具
type SkillMatchProcessor struct {
	searcher      SkillSearcher
	toolAssembler ToolAssembler
	topK          int // 返回最多 K 个技能
	logger        logging.Logger
}

// NewSkillMatchProcessor 创建技能匹配处理器
func NewSkillMatchProcessor(
	searcher SkillSearcher,
	toolAssembler ToolAssembler,
	topK int,
) *SkillMatchProcessor {
	if topK <= 0 {
		topK = 3 // 默认返回 3 个
	}

	return &SkillMatchProcessor{
		searcher:      searcher,
		toolAssembler: toolAssembler,
		topK:          topK,
		logger:        logging.GetSystemLogger().Named("skill_processor"),
	}
}

// Name 返回处理器名称
func (p *SkillMatchProcessor) Name() string {
	return "SkillMatchProcessor"
}

// Process 处理技能匹配
func (p *SkillMatchProcessor) Process(ctx context.Context, thinkCtx *entity.ThinkContext) error {
	// 1. 检查是否有意图（依赖 IntentProcessor）
	if thinkCtx.Intent == nil {
		p.logger.Debug("no intent found, skip skill matching")
		return nil
	}

	// 2. 构建搜索查询
	query := p.buildSearchQuery(thinkCtx)
	if query == "" {
		p.logger.Debug("empty search query, skip skill matching")
		return nil
	}

	p.logger.Debug("skill matching started",
		logging.String("query", query),
		logging.Int("topK", p.topK),
	)

	// 3. 使用混合检索搜索技能
	matches, err := p.searcher.Search(query, p.topK)
	if err != nil {
		// 技能匹配失败不影响核心功能，只记录警告
		p.logger.Warn("skill search failed",
			logging.Err(err),
		)
		return nil
	}

	// 4. 如果没有匹配的技能，直接返回
	if len(matches) == 0 {
		p.logger.Debug("no skills matched")
		return nil
	}

	// 5. 选择最优技能（第一个）
	bestMatch := matches[0]
	skill := bestMatch.Skill

	p.logger.Info("skill matched",
		logging.String("skill_name", skill.Name),
		logging.Float64("score", bestMatch.Score),
	)

	// 6. 转换为 SkillSOP
	thinkCtx.MatchedSkills = []*entity.SkillSOP{skill.ToSOP()}

	// 7. 动态组装工具
	tools, err := p.toolAssembler.AssembleTools(skill)
	if err != nil {
		p.logger.Error("tool assembly failed",
			logging.String("skill_name", skill.Name),
			logging.Err(err),
		)
		// 工具组装失败，返回错误（必需工具缺失）
		return err
	}

	thinkCtx.Tools = tools

	p.logger.Info("skill matching completed",
		logging.String("skill_name", skill.Name),
		logging.Float64("score", bestMatch.Score),
		logging.Int("tools_count", len(tools)),
	)

	return nil
}

// buildSearchQuery 构建搜索查询
func (p *SkillMatchProcessor) buildSearchQuery(thinkCtx *entity.ThinkContext) string {
	// 优先使用用户输入
	if thinkCtx.Input != "" {
		return thinkCtx.Input
	}

	// 回退到意图类型
	if thinkCtx.Intent != nil && thinkCtx.Intent.Type != "" {
		return thinkCtx.Intent.Type
	}

	return ""
}
