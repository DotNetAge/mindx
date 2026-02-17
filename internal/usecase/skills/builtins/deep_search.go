package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"mindx/internal/utils"
	"mindx/pkg/i18n"
	"os"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// DeepSearch 深度搜索
// 此搜索可以根据用户输入的问题，通过搜索引擎获取相关文章，然后使用LLM对文章内容进行总结。
func NewDeepSearch(baseUrl string, apiKey string, model string, langName string) func(params map[string]any) (string, error) {
	return func(params map[string]any) (string, error) {
		terms, ok := params["terms"].(string)
		if !ok {
			return "", fmt.Errorf("terms is not a string")
		}

		br, err := utils.NewBrowser("")
		if err != nil {
			fmt.Fprintf(os.Stderr, i18n.TWithData("browser.create_failed", map[string]interface{}{"Error": err.Error()})+"\n")
			os.Exit(1)
		}

		defer func() {
			br.Close()
			utils.StopChromeDriver()
		}()

		startTime := time.Now()

		results, err := br.Search(terms, 5)
		if err != nil {
			fmt.Fprintf(os.Stderr, i18n.TWithData("search.failed", map[string]interface{}{"Error": err.Error()})+"\n")
			os.Exit(1)
		}

		if len(results) > 20 {
			results = results[:20]
		}

		config := openai.DefaultConfig(apiKey)
		config.BaseURL = baseUrl
		client := openai.NewClientWithConfig(config)

		filteredResults, err := filterResultsWithLLM(client, terms, results, model)
		if err != nil {
			fmt.Fprintf(os.Stderr, i18n.TWithData("filter.failed", map[string]interface{}{"Error": err.Error()})+"\n")
			os.Exit(1)
		}

		var pageContents []PageContent
		for _, result := range filteredResults {
			openResult, err := br.Open(result.Link)
			if err != nil {
				fmt.Fprintf(os.Stderr, i18n.TWithData("open_page_failed", map[string]interface{}{"Link": result.Link, "Error": err.Error()})+"\n")
				continue
			}
			pageContents = append(pageContents, PageContent{
				URL:     result.Link,
				Title:   result.Title,
				Content: openResult.Content,
			})
		}

		if len(pageContents) == 0 {
			fmt.Fprintln(os.Stderr, i18n.T("no_page_opened"))
			os.Exit(1)
		}

		summary, err := summarizeWithLLM(client, terms, pageContents, model, langName)
		if err != nil {
			fmt.Fprintf(os.Stderr, i18n.TWithData("summarize_failed", map[string]interface{}{"Error": err.Error()})+"\n")
			os.Exit(1)
		}

		elapsed := time.Since(startTime)
		return getJSONSearchResult(summary, pageContents, elapsed), nil
	}
}

type PageContent struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type DeepSearchResult struct {
	Summary      string        `json:"summary"`
	PageContents []PageContent `json:"page_contents"`
	Elapsed      string        `json:"elapsed"`
	ElapsedMs    int64         `json:"elapsed_ms"`
}

func filterResultsWithLLM(client *openai.Client, query string, results []utils.SearchResult, model string) ([]utils.SearchResult, error) {
	prompt := fmt.Sprintf(getFilterPrompt(), query)

	for i, result := range results {
		prompt += fmt.Sprintf(`[%d] Title: %s
URL: %s
Description: %s

`, i+1, result.Title, result.Link, result.Description)
	}

	prompt += getFilterPromptEnd()

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	content := resp.Choices[0].Message.Content
	lines := strings.Split(content, "\n")

	var filtered []utils.SearchResult
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var index int
		if _, err := fmt.Sscanf(line, "%d", &index); err == nil && index > 0 && index <= len(results) {
			filtered = append(filtered, results[index-1])
			if len(filtered) >= 3 {
				break
			}
		}
	}

	if len(filtered) == 0 && len(results) > 0 {
		filtered = append(filtered, results[0])
	}

	return filtered, nil
}

func summarizeWithLLM(client *openai.Client, query string, pageContents []PageContent, model string, langName string) (string, error) {
	prompt := fmt.Sprintf(getSummarizePrompt(), query)

	for i, page := range pageContents {
		content := page.Content
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}

		prompt += fmt.Sprintf(`[%d] Title: %s
URL: %s
Content: %s

`, i+1, page.Title, page.URL, content)
	}

	prompt += getSummarizePromptEnd(langName)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func getJSONSearchResult(summary string, pageContents []PageContent, elapsed time.Duration) string {
	output := DeepSearchResult{
		Summary:      summary,
		PageContents: pageContents,
		Elapsed:      elapsed.String(),
		ElapsedMs:    elapsed.Milliseconds(),
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, i18n.TWithData("json.serialize_failed", map[string]interface{}{"Error": err.Error()})+"\n")
		os.Exit(1)
	}
	return string(data)
}

func getSummarizePromptEnd(langName string) string {
	return fmt.Sprintf(`Please provide a detailed summary:
1. What are these contents about, give a comprehensive summary after understanding
2. Please list the links from all read articles that best match the user's question (i.e., all the above article links)
3. Please output the content in %s`, langName)
}

func getFilterPrompt() string {
	return `Please analyze the following search results and select the top 3 results most relevant to the query "%s" (at least 1).

Search results:
`
}

func getFilterPromptEnd() string {
	return `Please output the filtered results in the following format:
1. First most relevant result number
2. Second most relevant result number
3. Third most relevant result number

Example:
1
3
5

Only output numbers, no other explanation.`
}

func getSummarizePrompt() string {
	return `Please read the following web content on my behalf, then summarize the most matching content found on the Internet for the user query "%s".
Please read each article carefully, understand its core information, and then give a comprehensive summary based on all the information.
Web content:
`
}
