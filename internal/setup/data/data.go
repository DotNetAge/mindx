package data

type ModelItem struct {
	Name        string
	Desc        string
	BaseURL     string
	CredRef     string
}

func (i ModelItem) Title() string       { return i.Name }
func (i ModelItem) Description() string { return i.Desc }
func (i ModelItem) FilterValue() string { return i.Name }

type PythonInfo struct {
	Detected bool
	Version  string
	VenvPath string
}

type WizardResult struct {
	SelectedModel string
	CredRef       string
	APIKey        string
	Err           error

	DaemonSetup   bool
	PythonSetup   bool
	PythonInfo    PythonInfo
	EmbedderModel string
	PathSetup     bool
}
