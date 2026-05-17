package data

type AgentInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Role        string `json:"role"`
	Model       string `json:"model"`
	IsDefault   bool   `json:"is_default"`
}

type WelcomeData struct {
	AppTitle   string
	Version    string
	AgentName  string
	Workspace  string
	SessionID  string
	ProjectDir string
	ModelName  string
}
