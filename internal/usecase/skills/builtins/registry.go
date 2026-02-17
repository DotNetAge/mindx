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
		mgr.RegisterInternalSkill("cron_add", cronProvider.CronAdd)
		mgr.RegisterInternalSkill("cron_list", cronProvider.CronList)
		mgr.RegisterInternalSkill("cron_delete", cronProvider.CronDelete)
		mgr.RegisterInternalSkill("cron_pause", cronProvider.CronPause)
		mgr.RegisterInternalSkill("cron_resume", cronProvider.CronResume)
	}

	if cfg != nil {
		deepSearchFn := NewDeepSearch(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.LangName)
		mgr.RegisterInternalSkill("deep_search", deepSearchFn)
	}
}
