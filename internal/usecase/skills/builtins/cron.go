package builtins

import (
	"encoding/json"
	"fmt"
	"mindx/internal/usecase/cron"
)

type CronSkillProvider struct {
	scheduler cron.Scheduler
}

func NewCronSkillProvider(scheduler cron.Scheduler) *CronSkillProvider {
	return &CronSkillProvider{scheduler: scheduler}
}

func (p *CronSkillProvider) CronAdd(params map[string]any) (string, error) {
	if p.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not initialized")
	}

	name, _ := params["name"].(string)
	cronExpr, _ := params["cron"].(string)
	skill, _ := params["skill"].(string)

	if name == "" || cronExpr == "" || skill == "" {
		return "", fmt.Errorf("name, cron, and skill are required")
	}

	jobParams := make(map[string]any)
	if p, ok := params["params"].(map[string]any); ok {
		jobParams = p
	} else if pStr, ok := params["params"].(string); ok {
		json.Unmarshal([]byte(pStr), &jobParams)
	}

	job := &cron.Job{
		Name:   name,
		Cron:   cronExpr,
		Skill:  skill,
		Params: jobParams,
	}

	id, err := p.scheduler.Add(job)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job added with ID: %s", id), nil
}

func (p *CronSkillProvider) CronList(params map[string]any) (string, error) {
	if p.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not initialized")
	}

	jobs, err := p.scheduler.List()
	if err != nil {
		return "", err
	}

	result, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func (p *CronSkillProvider) CronDelete(params map[string]any) (string, error) {
	if p.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not initialized")
	}

	id, _ := params["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if err := p.scheduler.Delete(id); err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job %s deleted", id), nil
}

func (p *CronSkillProvider) CronPause(params map[string]any) (string, error) {
	if p.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not initialized")
	}

	id, _ := params["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if err := p.scheduler.Pause(id); err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job %s paused", id), nil
}

func (p *CronSkillProvider) CronResume(params map[string]any) (string, error) {
	if p.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not initialized")
	}

	id, _ := params["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if err := p.scheduler.Resume(id); err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job %s resumed", id), nil
}
