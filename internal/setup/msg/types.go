package msg

type ModelSelectedMsg struct {
	Name    string
	BaseURL string
	CredRef string
	Desc    string
}

type APIKeySubmittedMsg struct {
	Key string
}

type DaemonDecisionMsg struct {
	Install bool
}

type PythonDecisionMsg struct {
	Setup   bool
	Version string
}

type MemoryDecisionMsg struct {
	Download bool
}

type DownloadProgressMsg struct {
	Current int64
	Total   int64
	File    string
	Done    bool
	Err     error
	Status  string
}

type PathDecisionMsg struct {
	AddToPath bool
}

type StepNextMsg struct{}
type StepPrevMsg struct{}
type SkipStepMsg struct{}
type WizardQuitMsg struct{}

type WizardResizeMsg struct {
	Width  int
	Height int
}
