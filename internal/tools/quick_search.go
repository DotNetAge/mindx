package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"

	"github.com/DotNetAge/goharness/tools"
)

// QuickSearch performs semantic search over the local knowledge base.
// It finds relevant code and documentation by meaning, not by exact text match.
// Use this BEFORE Grep when searching for where or how something is implemented.
type QuickSearch struct {
	serverURL string
}

// NewQuickSearch creates a QuickSearch tool backed by the MindStore WebAPI.
func NewQuickSearch(serverURL string) tools.FuncTool {
	return &QuickSearch{serverURL: serverURL}
}

func (t *QuickSearch) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "QuickSearch",
		Description: "高效语义搜索 — 按含义查找本项目内的代码和文档，速度远超 Grep 和 WebSearch。",
		Prompt: `按含义搜索知识库。将其视为"按语义的 grep" — 即使不知道精确关键词也能找到相关代码和文档。
知识库可能已有答案，检索速度与精度远优于 Grep 与 WebSearch，在找不到相关结果才考虑回退 Grep 或 WebSearch 使用。

要求知识库服务（mrag serve）已运行。`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "query",
				Type:        "string",
				Description: "自然语言查询 — 主题、问题或概念名称。",
				Required:    true,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "最大结果数（1-20，默认：5）。",
				Required:    false,
				Default:     float64(5),
			},
			{
				Name:        "targetDir",
				Type:        "string",
				Description: "目标目录，可将搜索限制在项目目录可以缩小搜索范围。",
				Required:    false,
			},
			{
				Name:        "showScore",
				Type:        "boolean",
				Description: "是否显示相似度分数（默认：true）。",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "showDocID",
				Type:        "boolean",
				Description: "是否显示文档 ID（默认：true）。",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "contentMax",
				Type:        "integer",
				Description: "内容最大显示长度（默认：500）。",
				Required:    false,
				Default:     float64(500),
			},
		},
	}
}

type queryRequest struct {
	Text       string `json:"text"`
	TopK       int    `json:"top_k"`
	FilterPath string `json:"filter_path"`
	Format     string `json:"format"`
	ShowScore  bool   `json:"show_score"`
	ShowDocID  bool   `json:"show_doc_id"`
	ContentMax int    `json:"content_max"`
}

type apiQueryResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// ── 防重复查询缓存 ───────────────────────────────────────────────────

// searchCache 记录已查询过的搜索条件，避免重复请求知识库 API。
// 缓存命中时不返回结果内容，而是告知模型引用此前返回的结果。
var searchCache sync.Map

// searchCacheKey 生成缓存 key。
func searchCacheKey(query, targetDir string, limit int, showScore, showDocID bool, contentMax int) string {
	return "search:" + query + "|" + targetDir + "|" + strconv.Itoa(limit) +
		"|" + strconv.FormatBool(showScore) + "|" + strconv.FormatBool(showDocID) + "|" + strconv.Itoa(contentMax)
}

// isSearchCached 检查指定搜索条件是否已查询过。
func isSearchCached(query, targetDir string, limit int, showScore, showDocID bool, contentMax int) bool {
	_, ok := searchCache.Load(searchCacheKey(query, targetDir, limit, showScore, showDocID, contentMax))
	return ok
}

// markSearchCached 标记指定搜索条件已查询过。
func markSearchCached(query, targetDir string, limit int, showScore, showDocID bool, contentMax int) {
	searchCache.Store(searchCacheKey(query, targetDir, limit, showScore, showDocID, contentMax), true)
}

// ansiPattern 匹配 ANSI CSI 转义序列（颜色/样式码，如 \x1b[1m、\x1b[33m、\x1b[0m）。
// MindStore terminal formatter 输出带这些码，面向人类终端；对 LLM 是纯噪音，
// 会污染上下文且浪费 token，返回前需剥离。
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI 移除字符串中的 ANSI 转义序列，返回纯文本。
func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func (t *QuickSearch) Execute(ctx context.Context, params map[string]any) (any, error) {
	queryStr, err := tools.ValidateRequiredString("QuickSearch", params, "query")
	if err != nil {
		return nil, err
	}
	if len(queryStr) < 2 {
		return nil, fmt.Errorf("%s", tools.GuideInvalidValue("QuickSearch", "query", queryStr, "提供至少 2 个字符的具体关键词（可使用组合词或英文关键词）后重试"))
	}

	limit := 5
	if raw, ok := getParam(params, "limit"); ok {
		if v, ok := tools.ToFloat64(raw); ok && v > 0 {
			limit = int(v)
			if limit > 20 {
				limit = 20
			}
		}
	}

	filterPath := ""
	if raw, ok := getParam(params, "targetDir"); ok {
		if v, ok := raw.(string); ok && v != "" {
			filterPath = v
		}
	}
	// if filterPath == "" {
	// 	if tc := tools.GetToolContext(ctx); tc != nil && tc.Session != nil {
	// 		filterPath = tc.Session.ProjectDir()
	// 	}
	// }

	// 解析显示选项
	showScore := true
	if raw, ok := getParam(params, "showScore"); ok {
		if v, ok := raw.(bool); ok {
			showScore = v
		}
	}
	showDocID := true
	if raw, ok := getParam(params, "showDocID"); ok {
		if v, ok := raw.(bool); ok {
			showDocID = v
		}
	}
	contentMax := 500
	if raw, ok := getParam(params, "contentMax"); ok {
		if v, ok := tools.ToFloat64(raw); ok && v > 0 {
			contentMax = int(v)
		}
	}

	// 防重复查询缓存：相同参数已查询过则告知模型引用此前的结果
	targetDir := filterPath
	if isSearchCached(queryStr, targetDir, limit, showScore, showDocID, contentMax) {
		return "搜索结果自上次查询以来未发生变化。本对话中此前 QuickSearch 的结果仍然有效，请引用此前的结果。", nil
	}

	// 构建请求体
	reqBody := queryRequest{
		Text:       queryStr,
		TopK:       limit,
		FilterPath: filterPath,
		Format:     "terminal",
		ShowScore:  showScore,
		ShowDocID:  showDocID,
		ContentMax: contentMax,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			"序列化快速搜索请求时失败",
			tools.WithErrDetail("搜索请求参数无法序列化为 JSON（可能包含异常数据）", err),
			"先自查：我传入的 query、limit、targetDir 等参数是否取值合法？确认后重新调用",
		), err)
	}

	// 调用 MindStore WebAPI /api/query
	url := t.serverURL + "/api/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			"构造发送给知识库服务的 HTTP 请求时失败",
			tools.WithErrDetail("无法构造 HTTP 请求（可能是 URL 格式非法或请求参数异常）", err),
			"先自查：知识库服务地址（serverURL）与查询参数配置是否正确？确认后重新调用",
		), err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			fmt.Sprintf("调用 QuickSearch 搜索 %q，但无法连接知识库服务", queryStr),
			tools.WithErrDetail("无法连接知识库服务（服务可能未启动或网络不可达）", err),
			"先启动 mrag serve 服务并确认端口与地址配置正确后重试；若服务持续不可用，应告知用户",
		), err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			fmt.Sprintf("调用 QuickSearch 搜索 %q 后读取响应失败", queryStr),
			tools.WithErrDetail("无法读取知识库服务返回的响应内容（连接可能在读取过程中中断）", err),
			"确认知识库服务已启动且网络连接稳定后重试；若仍失败应告知用户",
		), err)
	}

	var apiResp apiQueryResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
			fmt.Sprintf("调用 QuickSearch 搜索 %q 并解析响应时失败", queryStr),
			tools.WithErrDetail("知识库服务返回的响应不是合法的 JSON 格式", err),
			"确认知识库服务（mrag serve）版本与接口兼容、返回格式正确后重试；若仍失败应告知用户",
		), err)
	}

	if !apiResp.Success {
		cause := fmt.Sprintf("知识库服务返回错误（HTTP 状态码 %d）", resp.StatusCode)
		if apiResp.Error != "" {
			cause = tools.WithErrDetail(cause, errors.New(apiResp.Error))
		}
		return nil, fmt.Errorf("%s", tools.BuildGuide(
			fmt.Sprintf("调用 QuickSearch 搜索 %q，但知识库服务返回错误", queryStr),
			cause,
			"先自查：知识库服务是否已启动、查询参数是否正确？确认服务端正常后重试；若错误持续，应告知用户",
		))
	}

	emptyResultPrompt := tools.BuildGuide(
		"尝试搜索本地知识库，但没有找到相关的内容",
		"本地知识库中暂时没有任何与查询匹配的数据",
		"更换关键词或调整 targetDir 目录后重试；若仍无结果，可改用 Grep 或 WebSearch 搜索",
	)
	// 提取 formatted result
	if apiResp.Data == nil {
		return emptyResultPrompt, nil
	}

	result, _ := apiResp.Data["result"].(string)

	if result == "" {
		return emptyResultPrompt, nil
	}

	// 剥离 terminal formatter 的 ANSI 颜色码——这些码面向人类终端，对 LLM 是纯噪音，
	// 会污染上下文且浪费 token。
	result = stripANSI(result)
	markSearchCached(queryStr, targetDir, limit, showScore, showDocID, contentMax)
	return result, nil
}
