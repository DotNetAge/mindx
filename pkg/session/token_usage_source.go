package session

// UsageSource identifies where a token usage record originated.
//
// Sources are stored alongside the record in token_usages.yml and
// exposed via the token.usage.overview RPC response.
type UsageSource string

const (
	// UsageSourceChat indicates the token usage came from a chat/interaction.
	UsageSourceChat UsageSource = "chat"

	// UsageSourceIndexing indicates the token usage came from indexing.
	UsageSourceIndexing UsageSource = "indexing"

	// UsageSourceTranslation indicates the token usage came from translation.
	UsageSourceTranslation UsageSource = "translation"
)
