package cron

type Scheduler interface {
	Add(job *Job) (string, error)
	Delete(id string) error
	List() ([]*Job, error)
	Get(id string) (*Job, error)
	Pause(id string) error
	Resume(id string) error
	Update(id string, job *Job) error
	RunJob(id string) error
	UpdateLastRun(id string, status JobStatus, errMsg *string) error
}

type SkillInfoProvider interface {
	GetSkillInfo(name string) (isInternal bool, command string, dir string, err error)
}
