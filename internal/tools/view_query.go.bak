package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/events"
)

// viewQueryDefaultTimeout 是 ViewQuery 调用的默认超时。
const viewQueryDefaultTimeout = 30 * time.Second

// ViewQueryTool 实现了对 mindstore 视图查询 API 的结构化调用。
//
// 工作流程：
//  1. LLM 通过 action 参数选择 list_labels / describe / query 三种操作
//  2. 根据 action 构造对应的 HTTP 请求体
//  3. 调用 mindstore Web API（/api/view、/api/view/labels、/api/view/schema）
//  4. 将 JSON 响应原样返回给 LLM
//
// 适用场景：
//   - 当 Agent 需要列出知识库中所有可查询的标签时调用 list_labels
//   - 当 Agent 在编写查询前需要先了解目标标签的列结构时调用 describe
//   - 当 Agent 需要按条件分页查询实体集合时调用 query
type ViewQueryTool struct {
	// BaseURL 是 mindstore Web API 的基础地址（如 http://127.0.0.1:8484）。
	// 调用方必须显式注入，工具本身不做环境探测。
	BaseURL string

	// HTTPClient 允许调用方注入自定义 HTTP 客户端（用于鉴权、超时调整等）。
	HTTPClient *http.Client
}

// NewViewQueryTool 创建一个 ViewQueryTool 实例。
// baseURL 不能为空（如 http://127.0.0.1:8484）。
// 返回 nil 表示 baseURL 未配置。
func NewViewQueryTool(baseURL string) FuncTool {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	return &ViewQueryTool{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: viewQueryDefaultTimeout},
	}
}

// viewQueryRequest 视图查询请求体（与 mindstore Web API 一一对应）。
type viewQueryRequest struct {
	Label     string             `json:"label,omitempty"`
	Category  string             `json:"category,omitempty"`
	Fields    []string           `json:"fields,omitempty"`
	Where     *viewWhereRequest  `json:"where,omitempty"`
	OrderBy   []viewOrderRequest `json:"order_by,omitempty"`
	Page      int                `json:"page,omitempty"`
	Size      int                `json:"size,omitempty"`
	Path      string             `json:"path,omitempty"`
	WithTotal *bool              `json:"with_total,omitempty"`
}

// viewWhereRequest 过滤条件请求体。
type viewWhereRequest struct {
	Equals   map[string]any              `json:"equals,omitempty"`
	In       map[string][]any            `json:"in,omitempty"`
	Range    map[string]viewRangeRequest `json:"range,omitempty"`
	Contains map[string]string           `json:"contains,omitempty"`
}

// viewRangeRequest 范围请求体。
type viewRangeRequest struct {
	Gte any `json:"gte,omitempty"`
	Lte any `json:"lte,omitempty"`
}

// viewOrderRequest 排序请求体。
type viewOrderRequest struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc,omitempty"`
}

// Info 返回 ViewQuery 工具的元信息。
func (t *ViewQueryTool) Info() *ToolInfo {
	return &ToolInfo{
		Name:               "ViewQuery",
		MaxResultSizeChars: 30000,
		Description:        "结构化查询知识库中的视图（按 Schema 标签的实体集合）。",
		Prompt: `结构化查询知识库中的视图，提供"按标签分页列表实体"的能力。

**何时使用**
- 当你需要列出某种类型的全部实体（例如"所有分析师"），而语义搜索无法精确表达时。
- 当你需要按条件精确过滤（角色=分析师、所在组织=招商证券）时。
- 当你需要分页浏览结果时。

**三种动作（action 参数）**
1. action="list_labels"
   - 列出知识库中所有可查询的标签及其计数。
   - 不需要其他参数。
   - 返回示例：[{"label": "Person", "count": 12, "description": "员工/联系人"}, ...]

2. action="describe"
   - 获取指定标签的列结构（有哪些字段、类型、是否必填、说明）。
   - 必传 label 参数；可选 category。
   - 在编写 query 前必须先调用此动作了解可用列名。
   - 返回示例：{"label": "Person", "columns": [{"name": "name", "type": "string", "required": true}, ...]}

3. action="query"
   - 执行一次结构化分页查询。
   - 必传 label；可选 fields / where / order_by / page / size / category。
   - where 支持四类条件：equals（等值）、in（IN 列表）、range（闭区间 {gte, lte}）、contains（字符串子串）。
   - 默认每页 20 条，最多 200 条；page 从 1 开始。
   - 返回示例：{"label": "Person", "rows": [...], "total": 12, "page": 1, "size": 20}

**调用策略**
- 第一次使用 → 先 list_labels 探索可用标签
- 编写查询前 → describe 目标标签获取列名
- 然后 → 用 query 获取数据

**列名校验**
- 所有字段名必须来自 describe 返回的 columns.name 列表
- 非法字段名将返回错误码 40002

**性能说明**
- 视图查询走图数据库标签索引，O(N) 顺序扫，无 Embedding 成本
- 适合精确列举场景；不适合"语义相似的实体"搜索（请用 search 工具）`,
		Tags:          []string{"knowledge", "view", "query", "list", "schema"},
		IsReadOnly:    true,
		SecurityLevel: events.LevelSafe,
		Parameters: []Parameter{
			{
				Name:        "action",
				Type:        "string",
				Description: "动作类型：list_labels / describe / query。",
				Required:    true,
				Enum:        []any{"list_labels", "describe", "query"},
			},
			{
				Name:        "label",
				Type:        "string",
				Description: "节点标签名（如 \"Person\"、\"Asset\"）。action=describe/query 时必填。",
				Required:    false,
			},
			{
				Name:        "category",
				Type:        "string",
				Description: "Schema 类别（如 \"finance\"、\"general\"）。可选，缺省时按字典序自动选择。",
				Required:    false,
			},
			{
				Name:        "fields",
				Type:        "array",
				Description: "列白名单（字符串数组）。为空 = 返回所有列。仅 action=query 有效。",
				Required:    false,
			},
			{
				Name:        "where",
				Type:        "object",
				Description: "过滤条件 { equals?, in?, range?, contains? }。仅 action=query 有效。",
				Required:    false,
			},
			{
				Name:        "order_by",
				Type:        "array",
				Description: "排序规则 [{field, desc?}]。仅 action=query 有效。",
				Required:    false,
			},
			{
				Name:        "page",
				Type:        "integer",
				Description: "页码（1-based，默认 1）。仅 action=query 有效。",
				Required:    false,
			},
			{
				Name:        "size",
				Type:        "integer",
				Description: "每页条数（默认 20，上限 200）。仅 action=query 有效。",
				Required:    false,
			},
		},
	}
}

// Execute 执行视图查询。
//
// 参数处理：
//   - action 必填，取值 list_labels / describe / query
//   - list_labels：忽略其他参数
//   - describe：必填 label
//   - query：必填 label；可选 fields / where / order_by / page / size
//
// 返回：与 mindstore Web API 一致的 {code, message?, data} JSON。
func (t *ViewQueryTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	action, err := ValidateRequiredString("ViewQuery", params, "action")
	if err != nil {
		return nil, err
	}
	action = strings.TrimSpace(action)
	switch action {
	case "list_labels":
		return t.callHTTP(ctx, http.MethodGet, "/api/view/labels", nil)
	case "describe":
		label, err := ValidateRequiredString("ViewQuery", params, "label")
		if err != nil {
			return nil, err
		}
		category, _ := GetParam(params, "category")
		catStr, _ := category.(string)
		// GET /api/view/schema?label=X[&category=Y]
		path := "/api/view/schema?label=" + urlEncode(label)
		if catStr != "" {
			path += "&category=" + urlEncode(catStr)
		}
		return t.callHTTP(ctx, http.MethodGet, path, nil)
	case "query":
		return t.executeQuery(ctx, params)
	default:
		return nil, fmt.Errorf("%s", GuideInvalidValue("ViewQuery", "action", action, "先自查：action 只支持 list_labels / describe / query 三者之一，对照工具参数定义确认拼写后重新调用"))
	}
}

// executeQuery 构造并执行 action=query 的请求。
func (t *ViewQueryTool) executeQuery(ctx context.Context, params map[string]any) (any, error) {
	label, err := ValidateRequiredString("ViewQuery", params, "label")
	if err != nil {
		return nil, err
	}

	req := viewQueryRequest{Label: label}

	if v, ok := GetParam(params, "category"); ok {
		if s, ok2 := v.(string); ok2 {
			req.Category = s
		}
	}
	if v, ok := GetParam(params, "fields"); ok {
		if arr, ok2 := v.([]any); ok2 {
			for _, item := range arr {
				if s, ok3 := item.(string); ok3 {
					req.Fields = append(req.Fields, s)
				}
			}
		}
	}
	if v, ok := GetParam(params, "where"); ok {
		if m, ok2 := v.(map[string]any); ok2 {
			req.Where = convertWhereRequest(m)
		}
	}
	if v, ok := GetParam(params, "order_by"); ok {
		if arr, ok2 := v.([]any); ok2 {
			for _, item := range arr {
				if m, ok3 := item.(map[string]any); ok3 {
					order := viewOrderRequest{}
					if f, ok4 := m["field"].(string); ok4 {
						order.Field = f
					}
					if d, ok4 := m["desc"].(bool); ok4 {
						order.Desc = d
					}
					if order.Field != "" {
						req.OrderBy = append(req.OrderBy, order)
					}
				}
			}
		}
	}
	if v, ok := GetParam(params, "page"); ok {
		if f, ok2 := ToFloat64(v); ok2 {
			req.Page = int(f)
		}
	}
	if v, ok := GetParam(params, "size"); ok {
		if f, ok2 := ToFloat64(v); ok2 {
			req.Size = int(f)
		}
	}

	return t.callHTTP(ctx, http.MethodPost, "/api/view", req)
}

// convertWhereRequest 把 LLM 传入的 where 对象翻译为 viewWhereRequest。
// 输入：{"equals": {...}, "in": {...}, "range": {...}, "contains": {...}}
// 输出：归一化后的 viewWhereRequest。
func convertWhereRequest(m map[string]any) *viewWhereRequest {
	out := &viewWhereRequest{}
	if v, ok := m["equals"]; ok {
		if em, ok2 := v.(map[string]any); ok2 {
			out.Equals = em
		}
	}
	if v, ok := m["in"]; ok {
		if em, ok2 := v.(map[string]any); ok2 {
			out.In = make(map[string][]any, len(em))
			for k, list := range em {
				if arr, ok3 := list.([]any); ok3 {
					out.In[k] = arr
				}
			}
		}
	}
	if v, ok := m["range"]; ok {
		if em, ok2 := v.(map[string]any); ok2 {
			out.Range = make(map[string]viewRangeRequest, len(em))
			for k, item := range em {
				if rm, ok3 := item.(map[string]any); ok3 {
					r := viewRangeRequest{}
					if g, ok4 := rm["gte"]; ok4 {
						r.Gte = g
					}
					if l, ok4 := rm["lte"]; ok4 {
						r.Lte = l
					}
					out.Range[k] = r
				}
			}
		}
	}
	if v, ok := m["contains"]; ok {
		if em, ok2 := v.(map[string]any); ok2 {
			out.Contains = make(map[string]string, len(em))
			for k, item := range em {
				if s, ok3 := item.(string); ok3 {
					out.Contains[k] = s
				}
			}
		}
	}
	return out
}

// callHTTP 执行 HTTP 请求并返回 JSON 响应。
func (t *ViewQueryTool) callHTTP(ctx context.Context, method, path string, body any) (any, error) {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%s（原始错误：%w）", BuildGuide(
				"调用 ViewQuery 构造查询请求体",
				WithErrDetail("构造 mindstore API 请求时失败", err),
				"检查 action/label 等参数是否符合 ViewQuery 工具定义后重试",
			), err)
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, t.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", BuildGuide(
			fmt.Sprintf("调用 mindstore API（%s %s）时构造请求失败", method, path),
			WithErrDetail("构造 mindstore API 请求时失败", err),
			"检查 action/label 等参数是否符合 ViewQuery 工具定义后重试",
		), err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", BuildGuide(
			fmt.Sprintf("调用 mindstore API（%s %s）失败", method, path),
			WithErrDetail("无法连接 mindstore 服务", err),
			"先确认 mindstore 服务已启动、端口与地址配置正确；若服务不可达，应告知用户",
		), err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", BuildGuide(
			fmt.Sprintf("调用 mindstore API（%s %s）后读取响应失败", method, path),
			WithErrDetail("读取 mindstore 响应失败", err),
			"先确认 mindstore 服务已启动、端口与地址配置正确；若服务不可达，应告知用户",
		), err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", BuildGuide(
			fmt.Sprintf("调用 mindstore API（%s %s），但服务返回 HTTP %d", method, path, resp.StatusCode),
			WithErrDetail(fmt.Sprintf("mindstore 服务返回 HTTP %d 状态码", resp.StatusCode), errors.New(strings.TrimSpace(string(respBody)))),
			"先确认 mindstore 服务已启动、端口与地址配置正确；若服务不可达，应告知用户",
		))
	}

	// mindstore 响应统一是 {code, message, hint?, data} 结构，原样返回
	var out any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("%s（原始错误：%w）", BuildGuide(
			fmt.Sprintf("调用 mindstore API（%s %s）并解析响应", method, path),
			WithErrDetail("解析 mindstore 响应失败", err),
			"检查请求参数是否正确（如 label 是否存在），修正后重试",
		), err)
	}
	return out, nil
}

// urlEncode 用于构造 URL query string 的最小化实现。
func urlEncode(s string) string {
	// 简化处理：标签/类别名只能是 [A-Za-z0-9_]；保守起见，转义空格等
	s = strings.ReplaceAll(s, " ", "%20")
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, "?", "%3F")
	s = strings.ReplaceAll(s, "=", "%3D")
	return s
}
