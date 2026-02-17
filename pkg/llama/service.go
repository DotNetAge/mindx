package llama

type LlamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LlamaService interface {
	Chat(question string) (string, error)
	MultipleChat(questions []LlamaMessage) (string, error)
	ChatWithAgent(agent string, question string) (string, error) // agent = system prompt
}
