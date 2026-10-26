package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// 用户文章（社区文章发布与阅读）：文章发布到用户在社区的个人节点，组织节点
// 仅组织所有者可发。服务端路由（与账号/经验 API 同一 attachIdentity 通道；
// 官网文章读侧公开，匿名可访问）：
//
//	POST   /api/v1/articles               → 201 {slug,title,date}
//	DELETE /api/v1/articles/{slug}        → 204（仅作者）
//	GET    /api/v1/articles/{slug}        → {slug,title,summary,markdown,tags,date,org_id,scope}
//	                                        统一详情入口：官网文章匿名可读，个人文章仅作者、
//	                                        组织文章仅成员，无权/不存在 404（消息透传）
//	GET    /api/v1/articles?q=<kw>        → {"articles":[...]} 官网文章搜索（公开）
//	GET    /api/v1/articles?scope=mine&q=<kw>   → 我的文章搜索（含 org_id）
//	GET    /api/v1/orgs/{orgID}/articles?q=<kw> → 组织文章搜索（成员）
//	POST   /api/v1/orgs/{orgID}/articles  → 201（组织所有者）
//	DELETE /api/v1/orgs/{orgID}/articles/{slug} → 204（组织所有者）

// MarketArticle 用户文章条目（GET /api/v1/articles?scope=mine 清单项）。
type MarketArticle struct {
	Slug    string   `json:"slug"`
	Title   string   `json:"title"`
	Summary string   `json:"summary,omitempty"`
	Date    string   `json:"date,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	// OrgID 归属组织节点 ID（0 = 个人节点）。
	OrgID int64 `json:"org_id,omitempty"`
}

// ArticleDetail 文章详情（GET /api/v1/articles/{slug} 统一详情入口）。
type ArticleDetail struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Summary  string   `json:"summary,omitempty"`
	Markdown string   `json:"markdown"`
	Tags     []string `json:"tags,omitempty"`
	Date     string   `json:"date,omitempty"`
	// OrgID 归属组织节点 ID（0 = 非组织文章）。
	OrgID int64 `json:"org_id,omitempty"`
	// Scope 归属范围：site（官网公开）/ personal（个人节点）/ org（组织节点）。
	Scope string `json:"scope,omitempty"`
}

// ArticlePublishResult 文章发布响应（服务端 POST .../articles）。
type ArticlePublishResult struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// articleJSON 文章 API 专用 JSON 请求（POST/PUT/DELETE/GET 通用）：登录令牌鉴权
// （未登录直接报错，不发请求）；非 2xx 优先透传服务端的人话错误消息，404 按场景
// 归一（删除 = 文章不存在，其余 = 当前社区版本尚未提供文章功能）。
func (m *MarketClient) articleJSON(method, path string, body any, out any) error {
	if m.authToken == "" {
		return fmt.Errorf("尚未登录社区，请先绑定 MindX 账号")
	}
	return m.articleJSONOpen(method, path, body, out)
}

// articleJSONOpen 文章 API JSON 请求的匿名放行变体：官网文章读侧公开，未登录
// 也可访问（个人/组织文章的读写权限由服务端按令牌裁决，无权时原样返回错误）。
func (m *MarketClient) articleJSONOpen(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, m.apiBase+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	m.attachIdentity(req)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("社区暂时无法访问：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxSize))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusNotFound {
			if method == http.MethodDelete {
				return fmt.Errorf("这篇文章不存在，或它不属于你可管理的节点")
			}
			// 详情读取路径（GET /articles/{slug}）404 时透传服务端消息
			//（文章不存在或无权查看）；清单等其余接口 404 归一为社区版本未开放。
			if method == http.MethodGet && strings.HasPrefix(path, "/articles/") {
				var e struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
					return fmt.Errorf("%s", e.Error.Message)
				}
			}
			return fmt.Errorf("当前社区版本尚未开放文章功能，请稍后再试")
		}
		// 非 404 错误：透出服务端人话 message（回落见 apiErrorMessage）。
		return apiErrorMessage(data, resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("解析响应失败：%w", err)
		}
	}
	return nil
}

// articleBody 组装发布请求体（个人节点与组织节点共用；空可选字段不下发）。
func articleBody(slug, title, summary, markdown string, tags []string) map[string]any {
	body := map[string]any{"slug": slug, "title": title, "markdown": markdown}
	if summary = strings.TrimSpace(summary); summary != "" {
		body["summary"] = summary
	}
	if len(tags) > 0 {
		body["tags"] = tags
	}
	return body
}

// PublishArticle 发布文章到个人节点（POST /api/v1/articles；slug 已存在时由
// 服务端报错，调用方原样转达）。
func (m *MarketClient) PublishArticle(slug, title, summary, markdown string, tags []string) (*ArticlePublishResult, error) {
	var out ArticlePublishResult
	if err := m.articleJSON(http.MethodPost, "/articles", articleBody(slug, title, summary, markdown, tags), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PublishArticleToOrg 发布文章到组织节点（POST /api/v1/orgs/{orgID}/articles；
// 仅组织所有者，非所有者由服务端拒绝）。
func (m *MarketClient) PublishArticleToOrg(orgID int64, slug, title, summary, markdown string, tags []string) (*ArticlePublishResult, error) {
	var out ArticlePublishResult
	path := "/orgs/" + strconv.FormatInt(orgID, 10) + "/articles"
	if err := m.articleJSON(http.MethodPost, path, articleBody(slug, title, summary, markdown, tags), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// articleListQuery 组装文章清单查询串：scope 段在前，q 关键词可选（空串省略）。
func articleListQuery(scope, q string) string {
	vals := url.Values{}
	if scope != "" {
		vals.Set("scope", scope)
	}
	if q = strings.TrimSpace(q); q != "" {
		vals.Set("q", q)
	}
	if enc := vals.Encode(); enc != "" {
		return "?" + enc
	}
	return ""
}

// ReadArticle 读取文章详情（GET /api/v1/articles/{slug} 统一详情入口）：
// 官网文章匿名可读；个人文章仅作者、组织文章仅成员，无权或不存在时 404，
// 服务端的人话错误消息原样透传。
func (m *MarketClient) ReadArticle(slug string) (*ArticleDetail, error) {
	var out ArticleDetail
	path := "/articles/" + url.PathEscape(slug)
	if err := m.articleJSONOpen(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSiteArticles 搜索官网公开文章（GET /api/v1/articles?q=<kw>；公开，匿名可访问）。
func (m *MarketClient) ListSiteArticles(q string) ([]MarketArticle, error) {
	var out struct {
		Articles []MarketArticle `json:"articles"`
	}
	if err := m.articleJSONOpen(http.MethodGet, "/articles"+articleListQuery("", q), nil, &out); err != nil {
		return nil, err
	}
	return out.Articles, nil
}

// ListMyArticles 搜索我的文章清单（GET /api/v1/articles?scope=mine&q=<kw>）。
func (m *MarketClient) ListMyArticles(q string) ([]MarketArticle, error) {
	var out struct {
		Articles []MarketArticle `json:"articles"`
	}
	if err := m.articleJSON(http.MethodGet, "/articles"+articleListQuery("mine", q), nil, &out); err != nil {
		return nil, err
	}
	return out.Articles, nil
}

// ListOrgArticles 搜索组织节点文章（GET /api/v1/orgs/{orgID}/articles?q=<kw>；
// 组织成员可读，非成员由服务端拒绝）。
func (m *MarketClient) ListOrgArticles(orgID int64, q string) ([]MarketArticle, error) {
	var out struct {
		Articles []MarketArticle `json:"articles"`
	}
	path := "/orgs/" + strconv.FormatInt(orgID, 10) + "/articles" + articleListQuery("", q)
	if err := m.articleJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Articles, nil
}

// DeleteArticle 删除个人节点文章（DELETE /api/v1/articles/{slug}；仅作者，204）。
func (m *MarketClient) DeleteArticle(slug string) error {
	return m.articleJSON(http.MethodDelete, "/articles/"+url.PathEscape(slug), nil, nil)
}

// DeleteArticleInOrg 删除组织节点文章（DELETE /api/v1/orgs/{orgID}/articles/{slug}；
// 仅组织所有者，204）。
func (m *MarketClient) DeleteArticleInOrg(orgID int64, slug string) error {
	path := "/orgs/" + strconv.FormatInt(orgID, 10) + "/articles/" + url.PathEscape(slug)
	return m.articleJSON(http.MethodDelete, path, nil, nil)
}
