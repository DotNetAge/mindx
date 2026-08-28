package rpc

// CommitMessageParams 是 git.commit_message 的入参：前端收集的 git diff 文本。
type CommitMessageParams struct {
	Diff string `json:"diff"`
}

// CommitMessageResult 是 git.commit_message 的返回结果。
type CommitMessageResult struct {
	Message string `json:"message"`
}
