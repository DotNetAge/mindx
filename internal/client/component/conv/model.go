package conv

type Status int

const (
	StatusThinking Status = iota
	StatusExecuting
	StatusResponding
	StatusDone
	StatusError
)

type ActionStepStatus int

const (
	ActionStepExecuting ActionStepStatus = iota
	ActionStepDone
	ActionStepFailed
)
