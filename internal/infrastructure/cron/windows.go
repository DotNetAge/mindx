//go:build windows

package cron

import (
	"fmt"
	"mindx/internal/usecase/cron"
	"os"
	"os/exec"
	"strings"
)

type WindowsTaskScheduler struct {
	store             cron.JobStore
	skillInfoProvider cron.SkillInfoProvider
}

func NewWindowsTaskScheduler(skillInfoProvider cron.SkillInfoProvider) (cron.Scheduler, error) {
	store, err := NewFileJobStore()
	if err != nil {
		return nil, err
	}
	return &WindowsTaskScheduler{
		store:             store,
		skillInfoProvider: skillInfoProvider,
	}, nil
}

func (w *WindowsTaskScheduler) Add(job *cron.Job) (string, error) {
	id, err := w.store.Add(job)
	if err != nil {
		return "", err
	}

	taskName := getTaskName(id)
	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = "mindx.exe"
	}

	cmd := fmt.Sprintf(`%s cron run --job-id %s`, binaryPath, id)

	schtasksCmd := exec.Command(
		"schtasks",
		"/create",
		"/tn", taskName,
		"/tr", cmd,
		"/sc", "daily",
		"/st", "00:00",
		"/f",
	)

	if err := schtasksCmd.Run(); err != nil {
		w.store.Delete(id)
		return "", err
	}

	return id, nil
}

func (w *WindowsTaskScheduler) Delete(id string) error {
	taskName := getTaskName(id)
	cmd := exec.Command("schtasks", "/delete", "/tn", taskName, "/f")
	if err := cmd.Run(); err != nil {
		return err
	}
	return w.store.Delete(id)
}

func (w *WindowsTaskScheduler) List() ([]*cron.Job, error) {
	return w.store.List()
}

func (w *WindowsTaskScheduler) Get(id string) (*cron.Job, error) {
	return w.store.Get(id)
}

func (w *WindowsTaskScheduler) Pause(id string) error {
	if err := w.store.Pause(id); err != nil {
		return err
	}
	taskName := getTaskName(id)
	exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()
	return nil
}

func (w *WindowsTaskScheduler) Resume(id string) error {
	if err := w.store.Resume(id); err != nil {
		return err
	}

	taskName := getTaskName(id)
	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = "mindx.exe"
	}

	cmd := fmt.Sprintf(`%s cron run --job-id %s`, binaryPath, id)

	schtasksCmd := exec.Command(
		"schtasks",
		"/create",
		"/tn", taskName,
		"/tr", cmd,
		"/sc", "daily",
		"/st", "00:00",
		"/f",
	)

	return schtasksCmd.Run()
}

func (w *WindowsTaskScheduler) Update(id string, job *cron.Job) error {
	if err := w.store.Update(id, job); err != nil {
		return err
	}

	taskName := getTaskName(id)
	exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()

	updatedJob, err := w.store.Get(id)
	if err != nil {
		return err
	}

	if !updatedJob.Enabled {
		return nil
	}

	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = "mindx.exe"
	}

	cmd := fmt.Sprintf(`%s cron run --job-id %s`, binaryPath, id)

	schtasksCmd := exec.Command(
		"schtasks",
		"/create",
		"/tn", taskName,
		"/tr", cmd,
		"/sc", "daily",
		"/st", "00:00",
		"/f",
	)

	return schtasksCmd.Run()
}

func (w *WindowsTaskScheduler) RunJob(id string) error {
	job, err := w.store.Get(id)
	if err != nil {
		return err
	}

	if !job.Enabled {
		return nil
	}

	if err := w.store.UpdateLastRun(id, cron.JobStatusRunning, nil); err != nil {
		return err
	}

	return nil
}

func (w *WindowsTaskScheduler) UpdateLastRun(id string, status cron.JobStatus, errMsg *string) error {
	return w.store.UpdateLastRun(id, status, errMsg)
}

func getTaskName(id string) string {
	return fmt.Sprintf("MindX_%s", strings.Replace(id, "-", "_", -1))
}
