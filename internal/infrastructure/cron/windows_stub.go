//go:build !windows

package cron

import (
	"fmt"
	"mindx/internal/usecase/cron"
)

func NewWindowsTaskScheduler(skillInfoProvider cron.SkillInfoProvider) (cron.Scheduler, error) {
	return nil, fmt.Errorf("Windows Task Scheduler is only available on Windows")
}
