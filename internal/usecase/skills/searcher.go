package skills

import (
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"sort"
	"strings"
	"sync"
)

type SkillSearcher struct {
	embedding          *embedding.EmbeddingService
	logger             logging.Logger
	mu                 sync.RWMutex
	skills             map[string]*core.Skill
	skillInfos         map[string]*entity.SkillInfo
	toolKeywordVectors map[string][][]float64
}

func NewSkillSearcher(embedding *embedding.EmbeddingService, logger logging.Logger) *SkillSearcher {
	return &SkillSearcher{
		embedding:          embedding,
		logger:             logger.Named("SkillSearcher"),
		skills:             make(map[string]*core.Skill),
		skillInfos:         make(map[string]*entity.SkillInfo),
		toolKeywordVectors: make(map[string][][]float64),
	}
}

func (s *SkillSearcher) SetData(skills map[string]*core.Skill, infos map[string]*entity.SkillInfo, vectors map[string][][]float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skills = skills
	s.skillInfos = infos
	s.toolKeywordVectors = vectors
}

func (s *SkillSearcher) Search(keywords ...string) ([]*core.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(keywords) == 0 {
		return s.getAllSkills(), nil
	}

	s.logger.Debug(i18n.T("skill.search_start"),
		logging.String("keywords", fmt.Sprintf("%v", keywords)),
		logging.Int("skill_count", len(s.skills)),
		logging.Int("vector_count", len(s.toolKeywordVectors)),
		logging.Bool("has_embedding", s.embedding != nil))

	if s.embedding != nil && len(s.toolKeywordVectors) > 0 {
		return s.searchByVector(keywords)
	}

	s.logger.Warn(i18n.T("skill.fallback_to_keyword"))
	return s.searchByKeywords(keywords)
}

func (s *SkillSearcher) getAllSkills() []*core.Skill {
	result := make([]*core.Skill, 0, len(s.skills))
	for _, skill := range s.skills {
		result = append(result, skill)
	}
	return result
}

func (s *SkillSearcher) searchByVector(keywords []string) ([]*core.Skill, error) {
	s.logger.Info(i18n.T("skill.vector_search_start"),
		logging.String("keywords", fmt.Sprintf("%v", keywords)),
		logging.Int("indexed_skills", len(s.toolKeywordVectors)))

	var keywordVectors [][]float64
	for _, kw := range keywords {
		vec, err := s.embedding.GenerateEmbedding(kw)
		if err != nil {
			s.logger.Warn(i18n.T("skill.gen_search_vector_failed"), logging.String("keyword", kw), logging.Err(err))
			continue
		}
		keywordVectors = append(keywordVectors, vec)
		s.logger.Debug(i18n.T("skill.query_vector_generated"), logging.String("keyword", kw), logging.Int("vector_len", len(vec)))
	}

	if len(keywordVectors) == 0 {
		s.logger.Warn(i18n.T("skill.no_search_vectors"))
		return s.searchByKeywords(keywords)
	}

	type scoredSkill struct {
		skill      *core.Skill
		skillName  string
		score      float64
		matchCount int
	}

	var scoredSkills []scoredSkill

	for skillName, skillKwVectors := range s.toolKeywordVectors {
		if len(skillKwVectors) == 0 {
			s.logger.Debug(i18n.T("skill.skill_no_vectors"), logging.String("skill", skillName))
			continue
		}

		var totalScore float64
		matchCount := 0

		for _, queryVec := range keywordVectors {
			maxSimilarity := 0.0
			for _, skillKwVec := range skillKwVectors {
				similarity := CalculateCosineSimilarity(queryVec, skillKwVec)
				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
			totalScore += maxSimilarity
			if maxSimilarity > 0.5 {
				matchCount++
			}
		}

		avgScore := totalScore / float64(len(keywordVectors))

		skill, ok := s.skills[skillName]
		if !ok {
			s.logger.Warn(i18n.T("skill.skill_not_found"), logging.String("skill", skillName))
			continue
		}

		s.logger.Info(i18n.T("skill.similarity_calc"),
			logging.String("skill", skillName),
			logging.Float64("score", avgScore),
			logging.Int("match_count", matchCount),
			logging.Int("keyword_vectors", len(skillKwVectors)))

		scoredSkills = append(scoredSkills, scoredSkill{
			skill:      skill,
			skillName:  skillName,
			score:      avgScore,
			matchCount: matchCount,
		})
	}

	if len(scoredSkills) == 0 {
		s.logger.Debug(i18n.T("skill.no_skill_vectors"))
		return s.searchByKeywords(keywords)
	}

	sort.Slice(scoredSkills, func(i, j int) bool {
		if scoredSkills[i].score != scoredSkills[j].score {
			return scoredSkills[i].score > scoredSkills[j].score
		}
		return scoredSkills[i].matchCount > scoredSkills[j].matchCount
	})

	if len(scoredSkills) > 0 {
		maxScore := scoredSkills[0].score
		if maxScore < 0.6 {
			topN := 3
			if len(scoredSkills) < 3 {
				topN = len(scoredSkills)
			}

			result := make([]*core.Skill, 0, topN)
			for i := 0; i < topN; i++ {
				result = append(result, scoredSkills[i].skill)
			}

			s.logger.Debug(i18n.T("skill.vector_search_multi"),
				logging.String("keywords", fmt.Sprintf("%v", keywords)),
				logging.String("found", fmt.Sprintf("%d", len(result))),
				logging.Float64("maxScore", maxScore))

			return result, nil
		}
	}

	result := []*core.Skill{scoredSkills[0].skill}

	s.logger.Debug(i18n.T("skill.vector_search_best"),
		logging.String("keywords", fmt.Sprintf("%v", keywords)),
		logging.Float64("score", scoredSkills[0].score))

	return result, nil
}

func (s *SkillSearcher) searchByKeywords(keywords []string) ([]*core.Skill, error) {
	type scoredSkill struct {
		skill *core.Skill
		info  *entity.SkillInfo
		score int
	}

	var scoredSkills []scoredSkill

	for name, skill := range s.skills {
		info := s.skillInfos[name]
		if info == nil || info.Def == nil {
			continue
		}

		score := 0

		searchText := fmt.Sprintf("%s %s %s %s",
			info.Def.Name,
			info.Def.Description,
			strings.Join(info.Def.Tags, " "),
			info.Def.Category)

		for _, kw := range keywords {
			kwLower := strings.ToLower(strings.TrimSpace(kw))

			if strings.Contains(strings.ToLower(info.Def.Name), kwLower) {
				score += 3
			}

			if strings.Contains(strings.ToLower(info.Def.Description), kwLower) {
				score += 2
			}

			for _, tag := range info.Def.Tags {
				if strings.Contains(strings.ToLower(tag), kwLower) {
					score += 2
				}
			}

			if strings.Contains(strings.ToLower(info.Def.Category), kwLower) {
				score += 1
			}

			if strings.Contains(strings.ToLower(searchText), kwLower) {
				score += 1
			}
		}

		if score > 0 {
			scoredSkills = append(scoredSkills, scoredSkill{
				skill: skill,
				info:  info,
				score: score,
			})
		}
	}

	sort.Slice(scoredSkills, func(i, j int) bool {
		return scoredSkills[i].score > scoredSkills[j].score
	})

	result := make([]*core.Skill, 0, len(scoredSkills))
	for _, ss := range scoredSkills {
		result = append(result, ss.skill)
	}

	s.logger.Debug(i18n.T("skill.keyword_search_complete"),
		logging.String("keywords", fmt.Sprintf("%v", keywords)),
		logging.String("found", fmt.Sprintf("%d", len(result))))

	return result, nil
}

func (s *SkillSearcher) IsVectorTableEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.toolKeywordVectors) == 0
}

func CalculateCosineSimilarity(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return 0
	}

	var dotProduct, norm1, norm2 float64
	for i := 0; i < len(vec1); i++ {
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (norm1 * norm2)
}
