package handlers

import (
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/skills"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SkillsHandler struct {
	skillMgr *skills.SkillMgr
	logger   logging.Logger
}

func NewSkillsHandler(skillMgr *skills.SkillMgr) *SkillsHandler {
	logger := logging.GetSystemLogger().Named("skills_handler")
	return &SkillsHandler{
		skillMgr: skillMgr,
		logger:   logger,
	}
}

func (h *SkillsHandler) listSkills(c *gin.Context) {
	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	skillInfos := h.skillMgr.GetSkillInfos()
	skillsArray := make([]*entity.SkillInfo, 0, len(skillInfos))
	for _, info := range skillInfos {
		skillsArray = append(skillsArray, info)
	}

	// 返回重索引状态
	isReIndexing := h.skillMgr.IsReIndexing()
	reIndexError := h.skillMgr.GetReIndexError()
	reIndexErrorStr := ""
	if reIndexError != nil {
		reIndexErrorStr = reIndexError.Error()
	}

	c.JSON(http.StatusOK, gin.H{
		"skills":       skillsArray,
		"count":        len(skillsArray),
		"isReIndexing": isReIndexing,
		"reIndexError": reIndexErrorStr,
	})
}

func (h *SkillsHandler) getReIndexStatus(c *gin.Context) {
	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	isReIndexing := h.skillMgr.IsReIndexing()
	reIndexError := h.skillMgr.GetReIndexError()
	reIndexErrorStr := ""
	if reIndexError != nil {
		reIndexErrorStr = reIndexError.Error()
	}

	c.JSON(http.StatusOK, gin.H{
		"isReIndexing": isReIndexing,
		"reIndexError": reIndexErrorStr,
	})
}

func (h *SkillsHandler) triggerReIndex(c *gin.Context) {
	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	if h.skillMgr.IsReIndexing() {
		c.JSON(http.StatusConflict, gin.H{"error": "重索引已在进行中"})
		return
	}

	go func() {
		if err := h.skillMgr.ReIndex(); err != nil {
			h.logger.Error(i18n.T("adapter.manual_reindex_failed"), logging.Err(err))
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "重索引已在后台启动"})
}

func (h *SkillsHandler) getSkill(c *gin.Context) {
	name := c.Param("name")

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	info, exists := h.skillMgr.GetSkillInfo(name)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在", "name": name})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":  name,
		"skill": info,
	})
}

func (h *SkillsHandler) convertSkill(c *gin.Context) {
	name := c.Param("name")

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	h.logger.Info(i18n.T("adapter.convert_skill_request"), logging.String("name", name))

	if err := h.skillMgr.ConvertSkill(name); err != nil {
		h.logger.Error(i18n.T("adapter.convert_skill_failed"), logging.String("name", name), logging.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"name":  name,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "技能转换成功",
		"name":    name,
	})
}

func (h *SkillsHandler) installDependencies(c *gin.Context) {
	name := c.Param("name")

	var req struct {
		Binary string `json:"binary"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体"})
		return
	}

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	method := entity.InstallMethod{}

	if err := h.skillMgr.InstallDependency(name, method); err != nil {
		h.logger.Error(i18n.T("adapter.install_dep_failed"), logging.String("name", name), logging.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "安装依赖失败"})
		return
	}

	h.logger.Info(i18n.T("adapter.dep_installed"), logging.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "依赖已安装"})
}

func (h *SkillsHandler) installRuntime(c *gin.Context) {
	name := c.Param("name")

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	h.logger.Info(i18n.T("adapter.install_runtime_request"), logging.String("name", name))

	if err := h.skillMgr.InstallRuntime(name); err != nil {
		h.logger.Error(i18n.T("adapter.install_runtime_failed"), logging.String("name", name), logging.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"name":  name,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "运行时安装成功",
		"name":    name,
	})
}

func (h *SkillsHandler) getDependencies(c *gin.Context) {
	name := c.Param("name")

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	_, exists := h.skillMgr.GetSkillInfo(name)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在", "name": name})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":         name,
		"missing_bins": []string{},
	})
}

func (h *SkillsHandler) getEnv(c *gin.Context) {
	name := c.Param("name")

	env := map[string]string{}
	sensitiveKeys := getSensitiveKeys()

	for key, value := range env {
		for _, sensitiveKey := range sensitiveKeys {
			if containsSensitive(key, sensitiveKey) {
				env[key] = "***"
				break
			}
		}
		env[key] = value
	}

	c.JSON(http.StatusOK, gin.H{
		"name": name,
		"env":  env,
	})
}

func (h *SkillsHandler) setEnv(c *gin.Context) {
	name := c.Param("name")

	var env map[string]string
	if err := c.ShouldBindJSON(&env); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体"})
		return
	}

	h.logger.Info(i18n.T("adapter.set_env_request"), logging.String("name", name), logging.String("keys", formatMapKeys(env)))

	c.JSON(http.StatusOK, gin.H{"message": "环境变量已更新"})
}

func (h *SkillsHandler) validateSkill(c *gin.Context) {
	name := c.Param("name")

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	info, exists := h.skillMgr.GetSkillInfo(name)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在", "name": name})
		return
	}

	errors := []string{}
	if info.Def != nil {
		if info.Def.Description == "" {
			errors = append(errors, "描述为空")
		}
		if info.Def.Category == "" {
			errors = append(errors, "分类为空")
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":   name,
		"valid":  len(errors) == 0,
		"errors": errors,
	})
}

func (h *SkillsHandler) getStats(c *gin.Context) {
	name := c.Param("name")

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	info, exists := h.skillMgr.GetSkillInfo(name)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在", "name": name})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    name,
		"enabled": info.Def != nil && info.Def.Enabled,
		"tags":    getTags(info),
		"version": getVersion(info),
	})
}

func getTags(info *entity.SkillInfo) []string {
	if info != nil && info.Def != nil && info.Def.Tags != nil {
		return info.Def.Tags
	}
	return []string{}
}

func getVersion(info *entity.SkillInfo) string {
	if info != nil && info.Def != nil {
		return info.Def.Version
	}
	return ""
}

func (h *SkillsHandler) enableSkill(c *gin.Context) {
	name := c.Param("name")

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	if err := h.skillMgr.Enable(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info(i18n.T("adapter.skill_enabled"), logging.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "已启用"})
}

func (h *SkillsHandler) disableSkill(c *gin.Context) {
	name := c.Param("name")

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	if err := h.skillMgr.Disable(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info(i18n.T("adapter.skill_disabled"), logging.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "已禁用"})
}

func (h *SkillsHandler) executeSkill(c *gin.Context) {
	name := c.Param("name")

	var params map[string]any
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体"})
		return
	}

	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	skills, err := h.skillMgr.GetSkills()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取技能失败"})
		return
	}

	var targetSkill *core.Skill
	for _, skill := range skills {
		if skill.GetName() == name {
			targetSkill = skill
			break
		}
	}

	if targetSkill == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在", "name": name})
		return
	}

	if err := h.skillMgr.Execute(targetSkill, params); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":   name,
		"result": "技能执行成功",
		"params": params,
	})
}

func (h *SkillsHandler) batchConvert(c *gin.Context) {
	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	var req struct {
		Names []string `json:"names"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体"})
		return
	}

	if len(req.Names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未指定要转换的技能"})
		return
	}

	h.logger.Info(i18n.T("adapter.batch_convert_request"), logging.Int("count", len(req.Names)))

	success, failed := h.skillMgr.BatchConvert(req.Names)

	c.JSON(http.StatusOK, gin.H{
		"message":       "批量转换完成",
		"success_count": len(success),
		"failed_count":  len(failed),
		"success":       success,
		"failed":        failed,
	})
}

func (h *SkillsHandler) batchInstall(c *gin.Context) {
	if h.skillMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "技能管理器不可用"})
		return
	}

	var req struct {
		Names []string `json:"names"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体"})
		return
	}

	if len(req.Names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未指定要安装的技能"})
		return
	}

	h.logger.Info(i18n.T("adapter.batch_install_request"), logging.Int("count", len(req.Names)))

	success, failed := h.skillMgr.BatchInstall(req.Names)

	c.JSON(http.StatusOK, gin.H{
		"message":       "批量安装完成",
		"success_count": len(success),
		"failed_count":  len(failed),
		"success":       success,
		"failed":        failed,
	})
}

func getSensitiveKeys() []string {
	return []string{
		"api_key", "apikey", "secret", "token", "password",
		"private_key", "access_token", "refresh_token",
	}
}

func containsSensitive(key, sensitiveKey string) bool {
	return len(key) >= len(sensitiveKey) &&
		(key == sensitiveKey ||
			key[:len(sensitiveKey)] == sensitiveKey ||
			key[len(key)-len(sensitiveKey):] == sensitiveKey)
}

func formatMapKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return "{" + joinStrings(keys, ",") + "}"
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
