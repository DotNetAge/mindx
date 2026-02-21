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

func (p *CronSkillProvider) Cron(params map[string]any) (string, error) {
	if p.scheduler == nil {
		return "", fmt.Errorf("cron scheduler not initialized")
	}

	action, _ := params["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action is required: add, list, delete, pause, resume")
	}

	switch action {
	case "add":
		return p.cronAdd(params)
	case "list":
		return p.cronList(params)
	case "delete":
		return p.cronDelete(params)
	case "pause":
		return p.cronPause(params)
	case "resume":
		return p.cronResume(params)
	default:
		return "", fmt.Errorf("invalid action: %s. Valid actions: add, list, delete, pause, resume", action)
	}
}

func (p *CronSkillProvider) cronAdd(params map[string]any) (string, error) {
	name, _ := params["name"].(string)
	cronExpr, _ := params["cron"].(string)
	message, _ := params["message"].(string)

	if name == "" || cronExpr == "" || message == "" {
		return "", fmt.Errorf("name, cron, and message are required for add action")
	}

	job := &cron.Job{
		Name:    name,
		Cron:    cronExpr,
		Message: message,
	}

	id, err := p.scheduler.Add(job)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job added with ID: %s", id), nil
}

func (p *CronSkillProvider) cronList(params map[string]any) (string, error) {
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

func (p *CronSkillProvider) cronDelete(params map[string]any) (string, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required for delete action")
	}

	if err := p.scheduler.Delete(id); err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job %s deleted", id), nil
}

func (p *CronSkillProvider) cronPause(params map[string]any) (string, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required for pause action")
	}

	if err := p.scheduler.Pause(id); err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job %s paused", id), nil
}

func (p *CronSkillProvider) cronResume(params map[string]any) (string, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required for resume action")
	}

	if err := p.scheduler.Resume(id); err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job %s resumed", id), nil
}
