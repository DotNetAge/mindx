package cron

type JobStore interface {
	Add(job *Job) (string, error)
	Get(id string) (*Job, error)
	List() ([]*Job, error)
	Update(id string, job *Job) error
	Delete(id string) error
	Pause(id string) error
	Resume(id string) error
	UpdateLastRun(id string, status JobStatus, errMsg *string) error
}
