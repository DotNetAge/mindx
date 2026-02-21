package builtins

import (
	"mindx/internal/usecase/cron"
	"mindx/internal/usecase/skills"
)

type BuiltinConfig struct {
	BaseURL  string
	Model    string
	APIKey   string
	LangName string
}

func RegisterBuiltins(mgr *skills.SkillMgr, cfg *BuiltinConfig, cronScheduler cron.Scheduler) {
	mgr.RegisterInternalSkill("web_search", Search)
	mgr.RegisterInternalSkill("open_url", OpenURL)
	mgr.RegisterInternalSkill("write_file", WriteFile)

	if cronScheduler != nil {
		cronProvider := NewCronSkillProvider(cronScheduler)
		mgr.RegisterInternalSkill("cron", cronProvider.Cron)
	}

	if cfg != nil {
		deepSearchFn := NewDeepSearch(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.LangName)
		mgr.RegisterInternalSkill("deep_search", deepSearchFn)
	}
}
