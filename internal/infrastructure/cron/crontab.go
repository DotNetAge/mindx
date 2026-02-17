package cron

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mindx/internal/usecase/cron"
	"os/exec"
	"strings"
)

const cronJobPrefix = "# MINDX_CRON_JOB:"

type CrontabScheduler struct {
	store             cron.JobStore
	skillInfoProvider cron.SkillInfoProvider
}

func NewCrontabScheduler(skillInfoProvider cron.SkillInfoProvider) (cron.Scheduler, error) {
	store, err := NewFileJobStore()
	if err != nil {
		return nil, err
	}
	return &CrontabScheduler{
		store:             store,
		skillInfoProvider: skillInfoProvider,
	}, nil
}

func (c *CrontabScheduler) Add(job *cron.Job) (string, error) {
	if c.skillInfoProvider == nil {
		return "", fmt.Errorf("skill info provider not available")
	}

	isInternal, command, dir, err := c.skillInfoProvider.GetSkillInfo(job.Skill)
	if err != nil {
		return "", err
	}

	if isInternal {
		return "", fmt.Errorf("internal skills are not supported for cron jobs")
	}

	if command == "" {
		return "", fmt.Errorf("skill command not found")
	}

	fullCommand := c.buildFullCommand(command, dir, job.Params)
	job.Command = fullCommand

	id, err := c.store.Add(job)
	if err != nil {
		return "", err
	}

	cronLine := fmt.Sprintf("%s %s %s", cronJobPrefix+id, job.Cron, fullCommand)

	if err := addLineToCrontab(cronLine); err != nil {
		c.store.Delete(id)
		return "", err
	}

	return id, nil
}

func (c *CrontabScheduler) buildFullCommand(command string, dir string, params map[string]any) string {
	if params == nil || len(params) == 0 {
		return command
	}

	jsonParams, _ := json.Marshal(params)
	escapedParams := strings.ReplaceAll(string(jsonParams), "\"", "\\\"")

	parts := parseCommand(command)
	if len(parts) == 0 {
		return command
	}

	var buf bytes.Buffer
	if dir != "" {
		buf.WriteString(fmt.Sprintf("cd %s && ", shellEscape(dir)))
	}
	buf.WriteString(command)
	buf.WriteString(fmt.Sprintf(" << '%s'\n%s\n%s", "END_PARAMS", escapedParams, "END_PARAMS"))
	return buf.String()
}

func (c *CrontabScheduler) Delete(id string) error {
	if err := removeLineFromCrontab(cronJobPrefix + id); err != nil {
		return err
	}
	return c.store.Delete(id)
}

func (c *CrontabScheduler) List() ([]*cron.Job, error) {
	return c.store.List()
}

func (c *CrontabScheduler) Get(id string) (*cron.Job, error) {
	return c.store.Get(id)
}

func (c *CrontabScheduler) Pause(id string) error {
	return c.commentLineInCrontab(cronJobPrefix + id)
}

func (c *CrontabScheduler) Resume(id string) error {
	return c.uncommentLineInCrontab(cronJobPrefix + id)
}

func (c *CrontabScheduler) Update(id string, job *cron.Job) error {
	oldJob, err := c.store.Get(id)
	if err != nil {
		return err
	}

	if err := c.store.Update(id, job); err != nil {
		return err
	}

	if err := removeLineFromCrontab(cronJobPrefix + id); err != nil {
		c.store.Update(id, oldJob)
		return err
	}

	cronLine := fmt.Sprintf("%s %s %s", cronJobPrefix+id, job.Cron, job.Command)
	if err := addLineToCrontab(cronLine); err != nil {
		c.store.Update(id, oldJob)
		addLineToCrontab(fmt.Sprintf("%s %s %s", cronJobPrefix+id, oldJob.Cron, oldJob.Command))
		return err
	}

	return nil
}

func (c *CrontabScheduler) RunJob(id string) error {
	job, err := c.store.Get(id)
	if err != nil {
		return err
	}
	if job.Command == "" {
		return fmt.Errorf("job command not found")
	}

	cmd := exec.Command("bash", "-c", job.Command)
	return cmd.Run()
}

func (c *CrontabScheduler) UpdateLastRun(id string, status cron.JobStatus, errMsg *string) error {
	return c.store.UpdateLastRun(id, status, errMsg)
}

func (c *CrontabScheduler) commentLineInCrontab(prefix string) error {
	lines, err := getCrontabLines()
	if err != nil {
		return err
	}

	modified := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			if !strings.HasPrefix(strings.TrimSpace(line), "#"+prefix) {
				lines[i] = "#" + line
				modified = true
			}
		}
	}

	if !modified {
		return nil
	}

	return setCrontabLines(lines)
}

func (c *CrontabScheduler) uncommentLineInCrontab(prefix string) error {
	lines, err := getCrontabLines()
	if err != nil {
		return err
	}

	modified := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#"+prefix) {
			lines[i] = strings.TrimPrefix(line, "#")
			modified = true
		}
	}

	if !modified {
		return nil
	}

	return setCrontabLines(lines)
}

func getCrontabLines() ([]string, error) {
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []string{}, nil
		}
		return nil, fmt.Errorf("读取 crontab 失败: %w, 输出: %s", err, string(output))
	}

	return strings.Split(string(output), "\n"), nil
}

func setCrontabLines(lines []string) error {
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置 crontab 失败: %w, 输出: %s", err, string(output))
	}
	return nil
}

func addLineToCrontab(line string) error {
	lines, err := getCrontabLines()
	if err != nil {
		return err
	}

	for _, l := range lines {
		if strings.Contains(l, line) {
			return nil
		}
	}

	lines = append(lines, line)
	return setCrontabLines(lines)
}

func removeLineFromCrontab(prefix string) error {
	lines, err := getCrontabLines()
	if err != nil {
		return err
	}

	var newLines []string
	for _, line := range lines {
		if !strings.Contains(line, prefix) {
			newLines = append(newLines, line)
		}
	}

	return setCrontabLines(newLines)
}

func parseCommand(cmdStr string) []string {
	var parts []string
	var current strings.Builder
	var inQuote bool

	for i := 0; i < len(cmdStr); i++ {
		c := cmdStr[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
